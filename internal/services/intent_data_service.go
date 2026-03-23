package services

import (
	"encoding/json"
	"strings"

	"github.com/run-bigpig/jcp/internal/models"
)

// IntentDataService 意图驱动数据服务
type IntentDataService struct {
	f10Service        *F10Service
	newsService       *NewsService
	longHuBangService *LongHuBangService
}

// NewIntentDataService 创建意图驱动数据服务
func NewIntentDataService(
	f10Service *F10Service,
	newsService *NewsService,
	longHuBangService *LongHuBangService,
) *IntentDataService {
	return &IntentDataService{
		f10Service:        f10Service,
		newsService:       newsService,
		longHuBangService: longHuBangService,
	}
}

// BuildIntentContext 构建意图驱动数据上下文（JSON）
func (s *IntentDataService) BuildIntentContext(code string, query string) string {
	if code == "" || strings.TrimSpace(query) == "" {
		return ""
	}

	intents := detectIntents(query)
	if len(intents) == 0 {
		return ""
	}

	result := map[string]any{
		"intents": intents,
	}

	for _, intent := range intents {
		switch intent {
		case "valuation":
			if s.f10Service != nil {
				if trend, err := s.f10Service.GetValuationTrend(code, "1y"); err == nil {
					result["valuationTrend"] = trimValuationTrend(trend, 6)
				}
			}
		case "fundflow":
			if s.f10Service != nil {
				if flow, err := s.f10Service.GetFundFlowByCode(code); err == nil {
					result["fundFlowRecent"] = trimFundFlow(flow, 6)
				}
			}
		case "performance":
			if s.f10Service != nil {
				if events, err := s.f10Service.GetPerformanceEventsByCode(code); err == nil {
					result["performanceEvents"] = trimPerformance(events)
				}
				if financials, err := s.f10Service.GetFinancialStatementsByCode(code); err == nil {
					result["financialsLatest"] = map[string]any{
						"income":   firstMapAny(financials.Income),
						"balance":  firstMapAny(financials.Balance),
						"cashflow": firstMapAny(financials.Cashflow),
					}
				}
			}
		case "themes":
			if s.f10Service != nil {
				if themes, err := s.f10Service.GetCoreThemes(code); err == nil {
					result["coreThemes"] = trimCoreThemes(themes)
				}
			}
		case "risk":
			if s.f10Service != nil {
				if pledge, err := s.f10Service.GetEquityPledgeByCode(code); err == nil {
					result["equityPledge"] = firstMapAny(pledge.Records)
				}
				if lockup, err := s.f10Service.GetLockupReleaseByCode(code); err == nil {
					result["lockupRelease"] = firstMapAny(lockup.Records)
				}
				if changes, err := s.f10Service.GetShareholderChangesByCode(code); err == nil {
					result["shareholderChanges"] = firstMapAny(changes.Records)
				}
				if buyback, err := s.f10Service.GetStockBuybackByCode(code); err == nil {
					result["buyback"] = firstMapAny(buyback.Records)
				}
			}
		case "institution":
			if s.f10Service != nil {
				if holders, err := s.f10Service.GetInstitutionalHoldingsByCode(code); err == nil {
					result["institutionHolders"] = trimMapSlice(holders.TopHolders, 5)
					if len(holders.Controller) > 0 {
						result["controller"] = holders.Controller
					}
				}
				if holders, err := s.f10Service.GetShareholderNumbersByCode(code); err == nil {
					result["shareholderNumbers"] = holders.Latest
				}
			}
		case "news":
			if s.newsService != nil {
				if list, err := s.newsService.GetTelegraphList(); err == nil {
					result["news"] = trimMapSlice(listToMaps(list), 5)
				}
			}
		case "longhubang":
			if s.longHuBangService != nil {
				items, err := s.longHuBangService.GetLongHuBangList(50, 1, "")
				if err == nil && items != nil {
					code6 := extractCode6(code)
					if code6 != "" {
						for _, item := range items.Items {
							if item.Code == code6 {
								result["longhubang"] = item
								break
							}
						}
					}
				}
			}
		}
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(raw)
}

func detectIntents(query string) []string {
	q := strings.ToLower(query)
	var intents []string

	if matchAny(q, []string{"估值", "pe", "pb", "市盈", "市净", "高估", "低估", "贵", "便宜", "合理"}) {
		intents = append(intents, "valuation")
	}
	if matchAny(q, []string{"资金", "主力", "净流", "流入", "流出", "大单", "超大单"}) {
		intents = append(intents, "fundflow")
	}
	if matchAny(q, []string{"业绩", "财报", "利润", "营收", "净利", "毛利", "roe", "负债", "现金流", "基本面"}) {
		intents = append(intents, "performance")
	}
	if matchAny(q, []string{"题材", "概念", "板块", "热点", "情绪", "热度", "舆情"}) {
		intents = append(intents, "themes")
	}
	if matchAny(q, []string{"风险", "解禁", "质押", "回购", "增持", "减持", "股东", "公告"}) {
		intents = append(intents, "risk")
	}
	if matchAny(q, []string{"机构", "持仓", "股东户数"}) {
		intents = append(intents, "institution")
	}
	if matchAny(q, []string{"新闻", "快讯", "公告", "研报", "消息"}) {
		intents = append(intents, "news")
	}
	if matchAny(q, []string{"龙虎榜", "上榜"}) {
		intents = append(intents, "longhubang")
	}

	return intents
}

func matchAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func trimValuationTrend(trend models.F10ValuationTrend, keep int) map[string]any {
	out := map[string]any{
		"source":         trend.Source,
		"range":          trend.Range,
		"requestedRange": trend.RequestedRange,
		"fallback":       trend.Fallback,
	}
	if len(trend.PE) > 0 {
		out["pe"] = trimMapSlice(trend.PE, keep)
	}
	if len(trend.PB) > 0 {
		out["pb"] = trimMapSlice(trend.PB, keep)
	}
	if len(trend.PS) > 0 {
		out["ps"] = trimMapSlice(trend.PS, keep)
	}
	if len(trend.PCF) > 0 {
		out["pcf"] = trimMapSlice(trend.PCF, keep)
	}
	return out
}

func trimFundFlow(flow models.FundFlowSeries, keep int) map[string]any {
	out := map[string]any{
		"fields": flow.Fields,
	}
	if len(flow.Lines) > 0 {
		out["lines"] = trimStringSlice2(flow.Lines, keep)
	}
	if len(flow.Latest) > 0 {
		out["latest"] = flow.Latest
	}
	if len(flow.Labels) > 0 {
		out["labels"] = flow.Labels
	}
	return out
}

func trimPerformance(events models.PerformanceEvents) map[string]any {
	return map[string]any{
		"forecast": firstMapAny(events.Forecast),
		"express":  firstMapAny(events.Express),
		"schedule": firstMapAny(events.Schedule),
	}
}

func trimCoreThemes(themes models.F10CoreThemes) map[string]any {
	return map[string]any{
		"boardTypes":           trimMapSlice(themes.BoardTypes, 6),
		"themes":               trimMapSlice(themes.Themes, 6),
		"history":              trimMapSlice(themes.History, 6),
		"selectedBoardReasons": trimMapSlice(themes.SelectedBoardReasons, 6),
		"popularLeaders":       trimMapSlice(themes.PopularLeaders, 6),
	}
}

func trimMapSlice(items []map[string]any, keep int) []map[string]any {
	if keep <= 0 || len(items) <= keep {
		return items
	}
	return items[:keep]
}

func trimStringSlice2(items [][]string, keep int) [][]string {
	if keep <= 0 || len(items) <= keep {
		return items
	}
	return items[len(items)-keep:]
}

func listToMaps(list []Telegraph) []map[string]any {
	result := make([]map[string]any, 0, len(list))
	for _, item := range list {
		result = append(result, map[string]any{
			"time":    item.Time,
			"content": item.Content,
			"url":     item.URL,
		})
	}
	return result
}

func extractCode6(code string) string {
	var digits []rune
	for _, r := range code {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) < 6 {
		return ""
	}
	return string(digits[len(digits)-6:])
}

func firstMapAny(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}
