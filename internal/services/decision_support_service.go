package services

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/models"
)

const (
	decisionReviewMaxRecords = 240
	decisionEvaluateLagHours = 12
)

// DecisionTimingSignal 会前定时点融合信号
type DecisionTimingSignal struct {
	Score              float64 `json:"score"`
	Trend              string  `json:"trend"`
	ActionSuggestion   string  `json:"actionSuggestion"`
	PriceChangePercent float64 `json:"priceChangePercent"`
	MainNetInflow      float64 `json:"mainNetInflow"`
	TurnoverRate       float64 `json:"turnoverRate"`
	Amplitude          float64 `json:"amplitude"`
	UpcomingEventDays  int     `json:"upcomingEventDays"`
}

// DecisionRiskGuard 会前风险硬约束
type DecisionRiskGuard struct {
	HardBlocked    bool     `json:"hardBlocked"`
	HardRules      []string `json:"hardRules,omitempty"`
	Violations     []string `json:"violations,omitempty"`
	AllowedActions []string `json:"allowedActions,omitempty"`
}

// DecisionReviewStats 复盘与自纠摘要
type DecisionReviewStats struct {
	SampleCount      int      `json:"sampleCount"`
	WinCount         int      `json:"winCount"`
	LossCount        int      `json:"lossCount"`
	WinRate          float64  `json:"winRate"`
	ConsecutiveLoss  int      `json:"consecutiveLoss"`
	OpenEvaluations  int      `json:"openEvaluations"`
	AdjustmentAdvice string   `json:"adjustmentAdvice,omitempty"`
	WeakTags         []string `json:"weakTags,omitempty"`
}

// DecisionSupportContext 会前注入给多 agent 的约束上下文
type DecisionSupportContext struct {
	GeneratedAt string               `json:"generatedAt"`
	Timing      DecisionTimingSignal `json:"timing"`
	Risk        DecisionRiskGuard    `json:"risk"`
	Review      DecisionReviewStats  `json:"review"`
}

type decisionReviewRecord struct {
	Timestamp      int64   `json:"timestamp"`
	StockCode      string  `json:"stockCode"`
	StockName      string  `json:"stockName,omitempty"`
	Query          string  `json:"query,omitempty"`
	Summary        string  `json:"summary,omitempty"`
	Direction      string  `json:"direction"`
	EntryPrice     float64 `json:"entryPrice"`
	TimingScore    float64 `json:"timingScore"`
	TimingAction   string  `json:"timingAction,omitempty"`
	HardBlocked    bool    `json:"hardBlocked"`
	ViolationCount int     `json:"violationCount"`
	Evaluated      bool    `json:"evaluated"`
	EvaluatedAt    int64   `json:"evaluatedAt,omitempty"`
	ReturnPercent  float64 `json:"returnPercent,omitempty"`
	Outcome        string  `json:"outcome,omitempty"` // win/loss/flat
}

// DecisionSupportService 会前信号融合 + 风险硬约束 + 会后复盘
type DecisionSupportService struct {
	f10Service *F10Service
	storeDir   string
	mu         sync.Mutex
}

// NewDecisionSupportService 创建决策支持服务
func NewDecisionSupportService(dataDir string, f10Service *F10Service) *DecisionSupportService {
	dir := filepath.Join(dataDir, "decision-review")
	_ = os.MkdirAll(dir, 0755)
	return &DecisionSupportService{
		f10Service: f10Service,
		storeDir:   dir,
	}
}

// BuildContext 构建会前上下文，并返回可直接注入给 agent 的文本
func (s *DecisionSupportService) BuildContext(code string, stock models.Stock, query string, position *models.StockPosition) (DecisionSupportContext, string) {
	return s.BuildContextWithCorePack(code, stock, query, position, nil)
}

// BuildContextWithCorePack 使用已加载核心数据包构建会前上下文，避免重复请求。
func (s *DecisionSupportService) BuildContextWithCorePack(code string, stock models.Stock, query string, position *models.StockPosition, corePack *models.CoreDataPack) (DecisionSupportContext, string) {
	now := time.Now()
	ctx := DecisionSupportContext{
		GeneratedAt: now.Format("2006-01-02 15:04:05"),
	}

	var valuation models.StockValuation
	var flow models.FundFlowSeries
	var events models.PerformanceEvents
	if corePack != nil {
		valuation = corePack.Valuation
		flow = corePack.FundFlow
		events = corePack.Performance
		if stock.Symbol == "" {
			stock.Symbol = corePack.Stock.Symbol
		}
		if stock.Name == "" {
			stock.Name = corePack.Stock.Name
		}
		if stock.Price == 0 {
			stock.Price = corePack.Stock.Price
		}
		if stock.ChangePercent == 0 {
			stock.ChangePercent = corePack.Stock.ChangePercent
		}
	} else if s.f10Service != nil && strings.TrimSpace(code) != "" {
		if v, err := s.f10Service.GetValuationByCode(code); err == nil {
			valuation = v
		}
		if f, err := s.f10Service.GetFundFlowByCode(code); err == nil {
			flow = f
		}
		if e, err := s.f10Service.GetPerformanceEventsByCode(code); err == nil {
			events = e
		}
	}

	mainNetInflow, _ := pickNumeric(flow.Latest,
		"mainNetInflow", "MAIN_NET_INFLOW", "NET_INFLOW_MAIN", "ZL_JLR",
		"主力净流入", "主力净额", "主力净流入净额")
	upcomingDays := detectUpcomingEventDays(events)

	timing := DecisionTimingSignal{
		PriceChangePercent: stock.ChangePercent,
		MainNetInflow:      mainNetInflow,
		TurnoverRate:       pickFirstPositive(valuation.TurnoverRate),
		Amplitude:          pickFirstPositive(valuation.Amplitude),
		UpcomingEventDays:  upcomingDays,
	}
	timing.Score = scoreTiming(timing, valuation)
	timing.Trend = classifyTrend(timing.Score)
	timing.ActionSuggestion = suggestAction(timing.Score)

	risk := DecisionRiskGuard{
		HardRules: []string{
			"单票建议仓位上限 20%，禁止满仓或梭哈。",
			"若出现硬约束违规，不得给出加仓或重仓建议。",
			"给出买卖建议必须附带止损触发条件（价格或跌幅）。",
		},
	}

	if timing.PriceChangePercent <= -7 {
		risk.Violations = append(risk.Violations, "当日跌幅过大（<= -7%），禁止激进抄底。")
	}
	if timing.Amplitude >= 12 {
		risk.Violations = append(risk.Violations, "振幅过高（>= 12%），短线波动风险高。")
	}
	if timing.PriceChangePercent >= 5 && timing.MainNetInflow < 0 {
		risk.Violations = append(risk.Violations, "价格上冲但主力净流出，疑似拉高分歧。")
	}
	if timing.UpcomingEventDays >= 0 && timing.UpcomingEventDays <= 3 {
		risk.Violations = append(risk.Violations, "临近关键披露窗口（3天内），事件冲击风险高。")
	}
	if valuation.PETTM > 90 || valuation.PB > 10 {
		risk.Violations = append(risk.Violations, "估值处于高位区间，回撤风险抬升。")
	}

	risk.HardBlocked = len(risk.Violations) > 0
	if risk.HardBlocked {
		risk.AllowedActions = []string{"观望", "减仓", "止损"}
	} else if timing.Score >= 25 {
		risk.AllowedActions = []string{"观望", "小仓试探", "持有", "减仓"}
	} else {
		risk.AllowedActions = []string{"观望", "持有", "减仓"}
	}

	review := s.buildReviewStats(code, stock.Price)
	if position != nil && position.Shares > 0 && review.AdjustmentAdvice == "" && risk.HardBlocked {
		review.AdjustmentAdvice = "已有持仓且当前触发硬约束，优先控制回撤，避免逆势加仓。"
	}

	ctx.Timing = timing
	ctx.Risk = risk
	ctx.Review = review

	payload := map[string]any{
		"decisionSupport": ctx,
		"executionRules": []string{
			"若 risk.hardBlocked = true，禁止给出加仓、重仓、追涨建议。",
			"结论必须同时给出：建议动作、触发条件、止损或降仓条件。",
			"若样本不足或复盘胜率偏低，降低主观置信度并明确说明不确定性。",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ctx, ""
	}

	instruction := "【会前量化信号与风控闸门】以下是系统生成的统一信号，请据此讨论并严格执行硬约束：\n" + string(raw)
	return ctx, instruction
}

// RecordDecision 记录一次“信号-决策-结果”样本，用于后续复盘自纠
func (s *DecisionSupportService) RecordDecision(code string, stock models.Stock, query string, summary string, ctx DecisionSupportContext) {
	code = strings.TrimSpace(code)
	if code == "" || stock.Price <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.loadRecords(code)
	record := decisionReviewRecord{
		Timestamp:      time.Now().UnixMilli(),
		StockCode:      code,
		StockName:      stock.Name,
		Query:          trimText(query, 200),
		Summary:        trimText(summary, 500),
		Direction:      inferDirection(summary, ctx.Timing.ActionSuggestion),
		EntryPrice:     stock.Price,
		TimingScore:    ctx.Timing.Score,
		TimingAction:   ctx.Timing.ActionSuggestion,
		HardBlocked:    ctx.Risk.HardBlocked,
		ViolationCount: len(ctx.Risk.Violations),
	}
	records = append(records, record)
	s.evaluatePendingRecords(records, stock.Price)
	if len(records) > decisionReviewMaxRecords {
		records = records[len(records)-decisionReviewMaxRecords:]
	}
	s.saveRecords(code, records)
}

func (s *DecisionSupportService) buildReviewStats(code string, currentPrice float64) DecisionReviewStats {
	stats := DecisionReviewStats{
		AdjustmentAdvice: "样本不足，先以风控约束为主，降低结论置信度。",
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return stats
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := s.loadRecords(code)
	changed := s.evaluatePendingRecords(records, currentPrice)
	if changed {
		s.saveRecords(code, records)
	}

	if len(records) == 0 {
		stats.OpenEvaluations = 0
		return stats
	}

	recent := lastNRecords(records, 40)
	evaluated := make([]decisionReviewRecord, 0, len(recent))
	openCount := 0
	for _, record := range recent {
		if !record.Evaluated {
			openCount++
			continue
		}
		evaluated = append(evaluated, record)
	}
	stats.OpenEvaluations = openCount
	if len(evaluated) == 0 {
		return stats
	}

	stats.SampleCount = len(evaluated)
	for _, record := range evaluated {
		switch record.Outcome {
		case "win":
			stats.WinCount++
		case "loss":
			stats.LossCount++
		}
	}
	if stats.SampleCount > 0 {
		stats.WinRate = round2(float64(stats.WinCount) / float64(stats.SampleCount) * 100)
	}

	// 连续失效统计（从最新往前看）
	for i := len(evaluated) - 1; i >= 0; i-- {
		if evaluated[i].Outcome == "loss" {
			stats.ConsecutiveLoss++
			continue
		}
		break
	}

	if stats.WinRate >= 60 && stats.ConsecutiveLoss == 0 {
		stats.AdjustmentAdvice = "近期信号稳定，可维持现有信号权重，但继续执行硬风控。"
	} else if stats.WinRate >= 45 && stats.ConsecutiveLoss < 3 {
		stats.AdjustmentAdvice = "信号有效性一般，建议降低仓位弹性，强调确认后交易。"
	} else {
		stats.AdjustmentAdvice = "近期信号失效率偏高，建议下调进攻型信号权重，优先防守。"
		stats.WeakTags = append(stats.WeakTags, "timing", "confidence")
	}

	return stats
}

func (s *DecisionSupportService) evaluatePendingRecords(records []decisionReviewRecord, currentPrice float64) bool {
	if len(records) == 0 || currentPrice <= 0 {
		return false
	}

	now := time.Now().UnixMilli()
	changed := false
	minLag := int64(decisionEvaluateLagHours) * int64(time.Hour/time.Millisecond)
	for i := range records {
		record := &records[i]
		if record.Evaluated || record.EntryPrice <= 0 {
			continue
		}
		if now-record.Timestamp < minLag {
			continue
		}

		ret := (currentPrice - record.EntryPrice) / record.EntryPrice * 100
		outcome := evaluateOutcome(record.Direction, ret)
		record.Evaluated = true
		record.EvaluatedAt = now
		record.ReturnPercent = round2(ret)
		record.Outcome = outcome
		changed = true
	}
	return changed
}

func evaluateOutcome(direction string, ret float64) string {
	switch direction {
	case "bullish":
		if ret >= 1.0 {
			return "win"
		}
		if ret <= -1.0 {
			return "loss"
		}
	case "bearish":
		if ret <= -1.0 {
			return "win"
		}
		if ret >= 1.0 {
			return "loss"
		}
	default:
		if math.Abs(ret) <= 2.0 {
			return "win"
		}
	}
	return "flat"
}

func scoreTiming(timing DecisionTimingSignal, valuation models.StockValuation) float64 {
	score := clamp(timing.PriceChangePercent*1.8, -25, 25)

	switch {
	case timing.MainNetInflow >= 1e8:
		score += 25
	case timing.MainNetInflow >= 3e7:
		score += 15
	case timing.MainNetInflow > 0:
		score += 6
	case timing.MainNetInflow <= -1e8:
		score -= 25
	case timing.MainNetInflow <= -3e7:
		score -= 15
	case timing.MainNetInflow < 0:
		score -= 6
	}

	switch {
	case timing.Amplitude <= 4 && timing.Amplitude > 0:
		score += 10
	case timing.Amplitude <= 8 && timing.Amplitude > 0:
		score += 2
	case timing.Amplitude > 12:
		score -= 15
	case timing.Amplitude > 8:
		score -= 8
	}

	switch {
	case valuation.PETTM > 0 && valuation.PETTM <= 25:
		score += 10
	case valuation.PETTM > 25 && valuation.PETTM <= 45:
		score += 4
	case valuation.PETTM > 80:
		score -= 12
	case valuation.PETTM > 45:
		score -= 6
	}

	if timing.UpcomingEventDays >= 0 && timing.UpcomingEventDays <= 3 {
		score -= 15
	} else if timing.UpcomingEventDays <= 7 {
		score -= 8
	}

	return round2(clamp(score, -100, 100))
}

func classifyTrend(score float64) string {
	switch {
	case score >= 35:
		return "偏强"
	case score >= 10:
		return "中性偏强"
	case score <= -30:
		return "偏弱"
	case score <= -10:
		return "中性偏弱"
	default:
		return "震荡"
	}
}

func suggestAction(score float64) string {
	switch {
	case score >= 35:
		return "可考虑小仓试探，禁止重仓"
	case score >= 10:
		return "等待回踩确认后再参与"
	case score <= -30:
		return "以减仓和防守为主"
	case score <= -10:
		return "观望，等待信号修复"
	default:
		return "观望"
	}
}

func detectUpcomingEventDays(events models.PerformanceEvents) int {
	candidates := []string{
		findDateString(firstRecordMap(events.Schedule), "预计披露日期", "披露日期", "NOTICE_DATE", "REPORT_DATE"),
		findDateString(firstRecordMap(events.Forecast), "公告日期", "NOTICE_DATE", "REPORT_DATE"),
		findDateString(firstRecordMap(events.Express), "公告日期", "NOTICE_DATE", "REPORT_DATE"),
	}

	now := time.Now()
	best := -1
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if days, ok := diffDays(now, candidate); ok {
			if days >= 0 && (best == -1 || days < best) {
				best = days
			}
		}
	}
	return best
}

func firstRecordMap(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func findDateString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := record[key]; ok {
			if text := strings.TrimSpace(fmt.Sprintf("%v", value)); text != "" && text != "--" {
				return text
			}
		}
	}
	// 宽松匹配
	for key, value := range record {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "date") || strings.Contains(lower, "日期") || strings.Contains(lower, "披露") {
			text := strings.TrimSpace(fmt.Sprintf("%v", value))
			if text != "" && text != "--" {
				return text
			}
		}
	}
	return ""
}

func diffDays(now time.Time, value string) (int, bool) {
	value = strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			d := t.Sub(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
			return int(d.Hours() / 24), true
		}
	}
	return 0, false
}

func pickNumeric(record map[string]any, keys ...string) (float64, bool) {
	if len(record) == 0 {
		return 0, false
	}
	normalized := make(map[string]any, len(record))
	for key, value := range record {
		normalized[strings.ToLower(strings.TrimSpace(key))] = value
	}

	for _, key := range keys {
		if value, ok := normalized[strings.ToLower(strings.TrimSpace(key))]; ok {
			if num, ok := parseFloat(value); ok {
				return num, true
			}
		}
	}

	// 模糊匹配
	for key, value := range normalized {
		for _, candidate := range keys {
			candidate = strings.ToLower(strings.TrimSpace(candidate))
			if strings.Contains(key, candidate) {
				if num, ok := parseFloat(value); ok {
					return num, true
				}
			}
		}
	}

	return 0, false
}

func parseFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		text := strings.TrimSpace(v)
		if text == "" || text == "--" {
			return 0, false
		}
		scale := 1.0
		if strings.Contains(text, "亿") {
			scale = 1e8
			text = strings.ReplaceAll(text, "亿", "")
		} else if strings.Contains(text, "万") {
			scale = 1e4
			text = strings.ReplaceAll(text, "万", "")
		}
		text = strings.ReplaceAll(text, ",", "")
		text = strings.ReplaceAll(text, "%", "")
		num, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return 0, false
		}
		return num * scale, true
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", v))
		if text == "" || text == "<nil>" || text == "--" {
			return 0, false
		}
		text = strings.ReplaceAll(text, ",", "")
		num, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return 0, false
		}
		return num, true
	}
}

func pickFirstPositive(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func inferDirection(summary string, action string) string {
	text := strings.ToLower(strings.TrimSpace(summary))
	if text != "" {
		bullishKeys := []string{"买", "加仓", "看多", "做多", "增持", "看涨"}
		bearishKeys := []string{"卖", "减仓", "止损", "看空", "回避", "做空", "看跌"}
		for _, key := range bearishKeys {
			if strings.Contains(text, key) {
				return "bearish"
			}
		}
		for _, key := range bullishKeys {
			if strings.Contains(text, key) {
				return "bullish"
			}
		}
	}

	actionText := strings.ToLower(action)
	if strings.Contains(actionText, "减仓") || strings.Contains(actionText, "防守") || strings.Contains(actionText, "观望") {
		return "neutral"
	}
	if strings.Contains(actionText, "试探") || strings.Contains(actionText, "参与") {
		return "bullish"
	}
	return "neutral"
}

func trimText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func lastNRecords(records []decisionReviewRecord, n int) []decisionReviewRecord {
	if n <= 0 || len(records) <= n {
		return append([]decisionReviewRecord(nil), records...)
	}
	return append([]decisionReviewRecord(nil), records[len(records)-n:]...)
}

func (s *DecisionSupportService) recordPath(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "/", "_")
	code = strings.ReplaceAll(code, "\\", "_")
	return filepath.Join(s.storeDir, code+".json")
}

func (s *DecisionSupportService) loadRecords(code string) []decisionReviewRecord {
	path := s.recordPath(code)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return []decisionReviewRecord{}
	}
	var records []decisionReviewRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return []decisionReviewRecord{}
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp < records[j].Timestamp
	})
	return records
}

func (s *DecisionSupportService) saveRecords(code string, records []decisionReviewRecord) {
	path := s.recordPath(code)
	if len(records) > decisionReviewMaxRecords {
		records = records[len(records)-decisionReviewMaxRecords:]
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}
