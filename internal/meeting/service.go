package meeting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/run-bigpig/jcp/internal/adk"
	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/run-bigpig/jcp/internal/memory"
	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// 日志实例
var log = logger.New("Meeting")

// 超时配置常量
const (
	MeetingTimeout         = 5 * time.Minute  // 整个会议的最大时长
	AgentTimeout           = 90 * time.Second // 单个专家发言的最大时长
	ModeratorTimeout       = 60 * time.Second // 小韭菜分析/总结的最大时长
	ResponsesFallbackGrace = 45 * time.Second // Responses 主请求失败后给 fallback 预留的缓冲时间
	ModelCreationTimeout   = 10 * time.Second // 模型创建的最大时长
	DefaultAIRetryCount    = 3                // AI 请求默认重试次数
	MinConfigTimeout       = 15 * time.Second // 可配置超时最小值
	MaxConfigTimeout       = 30 * time.Minute // 可配置超时最大值
)

// 错误定义
var (
	ErrMeetingTimeout    = errors.New("会议超时，已返回部分结果")
	ErrModeratorTimeout  = errors.New("小韭菜响应超时")
	ErrNoAIConfig        = errors.New("未配置 AI 服务")
	ErrNoAgents          = errors.New("没有可用的专家")
	ErrEmptyAgentReply   = errors.New("agent 返回空内容")
	ErrInconsistentReply = errors.New("agent 输出前后不一致")

	agentRefMarkerPattern             = regexp.MustCompile(`\[(\d+)\]|【\s*\d+\s*】`)
	decisionSectionLinePattern        = regexp.MustCompile(`^\s*(?:[#>\-*]+\s*)?(?:\*\*)?(?:【|\[)?\s*(结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效)\s*(?:】|\])?(?:\*\*)?\s*[:：]?\s*(.*)$`)
	inlineDecisionHeadingPattern      = regexp.MustCompile(`([。！？；;])\s*(#{1,6}\s*(?:结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效))`)
	inlineBracketHeadingPattern       = regexp.MustCompile(`([。！？；;])\s*([【\[]\s*(?:结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效)\s*[】\]])`)
	inlineColonHeadingPattern         = regexp.MustCompile(`([。！？；;])\s*((?:结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效)\s*[:：])`)
	inlineDecisionHeadingLoosePattern = regexp.MustCompile(`([^\n])\s{2,}(#{1,6}\s*(?:结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效))`)
	inlineBracketHeadingLoosePattern  = regexp.MustCompile(`([^\n])\s{2,}([【\[]\s*(?:结论|综合结论|理由|共识与分歧|触发与风控|操作与风控|触发|风控|风险与止损|操作建议|操作策略|失效条件|失效)\s*[】\]])`)
	stockSymbolPattern                = regexp.MustCompile(`(?i)\b(?:sh|sz|bj)\d{6}\b`)
	sixDigitSymbolPattern             = regexp.MustCompile(`\b\d{6}\b`)
	numberedListPrefixPattern         = regexp.MustCompile(`^\s*(?:\d+[.)、]|[（(]\d+[)）]|[-*+])\s*`)
	fenceBlockPattern                 = regexp.MustCompile("(?s)```.*?```")
	inlineCodePattern                 = regexp.MustCompile("`([^`]+)`")
	conservativeActionKeywords        = []string{"观望", "持有", "减仓", "止损", "离场", "回避", "等待", "不追涨", "不买", "谨慎"}
	aggressiveActionKeywords          = []string{"买入", "加仓", "建仓", "介入", "追涨", "做多", "抄底", "上车"}
	aggressiveNegativePhrases         = []string{"不建议买", "不宜买", "不买", "不追涨", "暂不买", "不要买", "禁止买"}
	conditionalSentenceMarkers        = []string{"若", "如果", "当", "待", "一旦", "只有", "满足", "触发", "前提", "条件", "站上", "跌破", "回踩", "突破", "否则", "再", "才", "失效"}
)

// AIConfigResolver AI配置解析器函数类型
// 根据 AIConfigID 返回对应的 AI 配置，如果 ID 为空或找不到则返回默认配置
type AIConfigResolver func(aiConfigID string) *models.AIConfig

// SupplementContextBuilder 基于已发言内容补充数据的构建器
type SupplementContextBuilder func(stock models.Stock, query string, history []DiscussionEntry) string

// Service 会议室服务，编排多专家并行分析
type Service struct {
	modelFactory      *adk.ModelFactory
	toolRegistry      *tools.Registry
	mcpManager        *mcp.Manager
	memoryManager     *memory.Manager
	memoryAIConfig    *models.AIConfig // 记忆管理使用的 LLM 配置
	moderatorAIConfig *models.AIConfig // 意图分析(小韭菜)使用的 LLM 配置
	aiConfigResolver  AIConfigResolver // AI配置解析器
	supplementBuilder SupplementContextBuilder
	retryCount        int // AI 请求重试次数
	verboseAgentIO    bool
	selectionStyle    models.AgentSelectionStyle
	enableSecondRound bool
}

type llmCreateFn func(context.Context, *models.AIConfig) (model.LLM, error)

type llmCreateResult struct {
	llm model.LLM
	err error
}

// llmPool 在单次会议请求内复用同配置模型，避免重复创建消耗。
type llmPool struct {
	create llmCreateFn

	defaultKey string
	defaultLLM model.LLM

	mu      sync.Mutex
	cache   map[string]model.LLM
	pending map[string][]chan llmCreateResult
}

func newLLMPool(defaultConfig *models.AIConfig, defaultLLM model.LLM, create llmCreateFn) *llmPool {
	p := &llmPool{
		create:     create,
		defaultKey: aiConfigPoolKey(defaultConfig),
		defaultLLM: defaultLLM,
		cache:      make(map[string]model.LLM),
		pending:    make(map[string][]chan llmCreateResult),
	}
	if p.defaultKey != "" && p.defaultLLM != nil {
		p.cache[p.defaultKey] = p.defaultLLM
	}
	return p
}

func (p *llmPool) get(ctx context.Context, cfg *models.AIConfig) (model.LLM, error) {
	if cfg == nil {
		if p.defaultLLM != nil {
			return p.defaultLLM, nil
		}
		return nil, ErrNoAIConfig
	}

	key := aiConfigPoolKey(cfg)
	if key == "" {
		if p.defaultLLM != nil {
			return p.defaultLLM, nil
		}
		return nil, ErrNoAIConfig
	}

	p.mu.Lock()
	if llm, ok := p.cache[key]; ok {
		p.mu.Unlock()
		return llm, nil
	}

	if waiters, ok := p.pending[key]; ok {
		ch := make(chan llmCreateResult, 1)
		p.pending[key] = append(waiters, ch)
		p.mu.Unlock()
		select {
		case result := <-ch:
			return result.llm, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ch := make(chan llmCreateResult, 1)
	p.pending[key] = []chan llmCreateResult{ch}
	p.mu.Unlock()

	llm, err := p.create(ctx, cfg)

	p.mu.Lock()
	waiters := p.pending[key]
	delete(p.pending, key)
	if err == nil && llm != nil {
		p.cache[key] = llm
	}
	p.mu.Unlock()

	result := llmCreateResult{llm: llm, err: err}
	for _, waiter := range waiters {
		waiter <- result
		close(waiter)
	}
	return llm, err
}

func aiConfigPoolKey(cfg *models.AIConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.ID != "" {
		return "id:" + cfg.ID
	}
	return fmt.Sprintf(
		"anon:%s|%s|%s|%s|%v|%s|%s|%s",
		cfg.Provider,
		cfg.Name,
		cfg.ModelName,
		cfg.BaseURL,
		cfg.UseResponses,
		cfg.Project,
		cfg.Location,
		cfg.APIKey,
	)
}

// NewServiceFull 创建完整配置的会议室服务
func NewServiceFull(registry *tools.Registry, mcpMgr *mcp.Manager) *Service {
	return &Service{
		modelFactory:      adk.NewModelFactory(),
		toolRegistry:      registry,
		mcpManager:        mcpMgr,
		retryCount:        DefaultAIRetryCount,
		verboseAgentIO:    true,
		selectionStyle:    models.AgentSelectionBalanced,
		enableSecondRound: true,
	}
}

// SetMemoryManager 设置记忆管理器
func (s *Service) SetMemoryManager(memMgr *memory.Manager) {
	s.memoryManager = memMgr
}

// SetMemoryAIConfig 设置记忆管理使用的 LLM 配置
func (s *Service) SetMemoryAIConfig(aiConfig *models.AIConfig) {
	s.memoryAIConfig = aiConfig
}

// SetModeratorAIConfig 设置意图分析(小韭菜)使用的 LLM 配置
func (s *Service) SetModeratorAIConfig(aiConfig *models.AIConfig) {
	s.moderatorAIConfig = aiConfig
}

// SetAIConfigResolver 设置 AI 配置解析器
func (s *Service) SetAIConfigResolver(resolver AIConfigResolver) {
	s.aiConfigResolver = resolver
}

// SetSupplementContextBuilder 设置补充数据上下文构建器
func (s *Service) SetSupplementContextBuilder(builder SupplementContextBuilder) {
	s.supplementBuilder = builder
}

// SetRetryCount 设置 AI 请求重试次数（1-5，超出范围自动收敛）
func (s *Service) SetRetryCount(count int) {
	if count < 1 {
		count = DefaultAIRetryCount
	}
	if count > 5 {
		count = 5
	}
	s.retryCount = count
}

// SetVerboseAgentIO 设置是否输出完整 Agent 请求/响应日志。
func (s *Service) SetVerboseAgentIO(enabled bool) {
	s.verboseAgentIO = enabled
}

// SetAgentSelectionStyle 设置小韭菜选人风格。
func (s *Service) SetAgentSelectionStyle(style models.AgentSelectionStyle) {
	switch style {
	case models.AgentSelectionBalanced, models.AgentSelectionConservative, models.AgentSelectionAggressive:
		s.selectionStyle = style
	default:
		s.selectionStyle = models.AgentSelectionBalanced
	}
}

// SetEnableSecondReview 设置是否启用二轮复议（补充数据校正轮）。
func (s *Service) SetEnableSecondReview(enabled bool) {
	s.enableSecondRound = enabled
}

func (s *Service) shouldRetry(err error, ctx context.Context) bool {
	if err == nil {
		return false
	}
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if errors.Is(err, ErrEmptyAgentReply) || errors.Is(err, ErrInconsistentReply) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "beta-limitations") || strings.Contains(msg, "not supported") {
		return false
	}

	retryHints := []string{
		"timeout",
		"time-out",
		"timed out",
		"temporarily unavailable",
		"connection reset",
		"connection refused",
		"connection closed",
		"no such host",
		"network error",
		"eof",
		"gateway",
		" 429",
		" 502",
		" 503",
		" 504",
		"status 429",
		"status 502",
		"status 503",
		"status 504",
		"rate limit",
	}
	for _, hint := range retryHints {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}

func (s *Service) waitBeforeRetry(ctx context.Context, attempt int) bool {
	if attempt < 1 {
		attempt = 1
	}
	base := 500 * time.Millisecond
	backoff := time.Duration(attempt) * base
	if backoff > 3*time.Second {
		backoff = 3 * time.Second
	}
	select {
	case <-time.After(backoff):
		return true
	case <-ctx.Done():
		return false
	}
}

func (s *Service) analyzeWithRetry(ctx context.Context, moderator *Moderator, stock *models.Stock, query string, agents []models.AgentConfig) (*ModeratorDecision, error) {
	var lastErr error
	for attempt := 1; attempt <= s.retryCount; attempt++ {
		decision, err := moderator.Analyze(ctx, stock, query, agents)
		if err == nil {
			return decision, nil
		}
		lastErr = err
		if !s.shouldRetry(err, ctx) || attempt == s.retryCount {
			return nil, err
		}
		log.Warn("moderator analyze error, retrying %d/%d: %v", attempt, s.retryCount, err)
		if !s.waitBeforeRetry(ctx, attempt) {
			return nil, err
		}
	}
	return nil, lastErr
}

func (s *Service) summarizeWithRetry(ctx context.Context, moderator *Moderator, stock *models.Stock, query string, history []DiscussionEntry, extraContext string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= s.retryCount; attempt++ {
		summary, err := moderator.SummarizeWithContext(ctx, stock, query, history, extraContext)
		if err == nil {
			cleaned := sanitizeAgentOutput(summary)
			if strings.TrimSpace(cleaned) == "" {
				err = fmt.Errorf("%w: agent=moderator", ErrEmptyAgentReply)
			} else if validateErr := validateAgentOutputConsistency(cleaned); validateErr != nil {
				err = validateErr
			} else {
				return cleaned, nil
			}
		}
		lastErr = err
		if !s.shouldRetry(err, ctx) || attempt == s.retryCount {
			return "", err
		}
		log.Warn("moderator summarize error, retrying %d/%d: %v", attempt, s.retryCount, err)
		if !s.waitBeforeRetry(ctx, attempt) {
			return "", err
		}
	}
	return "", lastErr
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Stock         models.Stock          `json:"stock"`
	KLineData     []models.KLineData    `json:"klineData"`
	Agents        []models.AgentConfig  `json:"agents"`
	Query         string                `json:"query"`
	ReplyContent  string                `json:"replyContent"`
	AllAgents     []models.AgentConfig  `json:"allAgents"` // 所有可用专家（智能模式用）
	CoreContext   string                `json:"coreContext"`
	IntentContext string                `json:"intentContext"`
	Position      *models.StockPosition `json:"position"` // 用户持仓信息
}

// ChatResponse 聊天响应
type ChatResponse struct {
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Round     int    `json:"round"`
	MsgType   string `json:"msgType"` // opening/opinion/summary
}

// ResponseCallback 响应回调函数类型
// 每当有新的发言产生时调用，用于实时推送到前端
type ResponseCallback func(resp ChatResponse)

// ProgressEvent 进度事件（细粒度实时反馈）
type ProgressEvent struct {
	Type      string `json:"type"`      // thinking/tool_call/tool_result/streaming/agent_start/agent_done
	AgentID   string `json:"agentId"`   // 当前专家 ID
	AgentName string `json:"agentName"` // 当前专家名称
	Detail    string `json:"detail"`    // 工具名称或阶段描述
	Content   string `json:"content"`   // 流式文本片段或工具结果摘要
}

// ProgressCallback 进度回调函数类型
type ProgressCallback func(event ProgressEvent)

func buildAgentFailureContent(err error) string {
	reason := "未产出有效内容"
	switch {
	case err == nil:
		reason = "返回空文本"
	case errors.Is(err, context.DeadlineExceeded):
		reason = "请求超时"
	case errors.Is(err, ErrEmptyAgentReply):
		reason = "返回空文本"
	default:
		reason = normalizeLogText(err.Error(), 80)
	}
	return fmt.Sprintf("本轮未产出有效观点（%s）。建议稍后重试，或切换模型/延长超时后再试。", reason)
}

type agentRunMetrics struct {
	eventCount       int
	nilEventCount    int
	llmRespCount     int
	partialCount     int
	turnComplete     int
	textPartCount    int
	textBytes        int
	thoughtPartCount int
	toolCallCount    int
	toolResultCount  int
	finishReason     string
}

func (m agentRunMetrics) summary() string {
	return fmt.Sprintf(
		"events=%d nil=%d llm=%d partial=%d turnComplete=%d textParts=%d textBytes=%d thoughtParts=%d toolCalls=%d toolResults=%d finishReason=%s",
		m.eventCount,
		m.nilEventCount,
		m.llmRespCount,
		m.partialCount,
		m.turnComplete,
		m.textPartCount,
		m.textBytes,
		m.thoughtPartCount,
		m.toolCallCount,
		m.toolResultCount,
		normalizeLogText(m.finishReason, 64),
	)
}

func normalizeLogText(input string, max int) string {
	if max <= 0 {
		max = 256
	}
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > max {
		return text[:max] + "...(truncated)"
	}
	return text
}

// mergeStreamText 合并流式文本，兼容增量片段与“累计全文”片段两种返回方式。
// 返回值:
//   - merged: 合并后的完整文本
//   - delta: 新增增量（用于实时推送）
//   - changed: 是否有新增内容
func mergeStreamText(current string, incoming string) (merged string, delta string, changed bool) {
	if incoming == "" {
		return current, "", false
	}
	if current == "" {
		return incoming, incoming, true
	}
	if strings.HasPrefix(incoming, current) {
		delta = incoming[len(current):]
		if delta == "" {
			return current, "", false
		}
		return incoming, delta, true
	}
	if strings.HasSuffix(current, incoming) {
		return current, "", false
	}
	if overlap := streamOverlapLen(current, incoming); overlap > 0 {
		delta = incoming[overlap:]
		if delta == "" {
			return current, "", false
		}
		return current + delta, delta, true
	}
	return current + incoming, incoming, true
}

func streamOverlapLen(left string, right string) int {
	max := len(left)
	if len(right) < max {
		max = len(right)
	}
	for k := max; k > 0; k-- {
		if left[len(left)-k:] == right[:k] {
			return k
		}
	}
	return 0
}

func sanitizeAgentOutput(content string) string {
	text := strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
	if text == "" {
		return ""
	}
	text = stripReferenceSection(text)
	text = agentRefMarkerPattern.ReplaceAllString(text, "")
	text = stripCodeFormatting(text)
	text = dedupeRepeatedTail(text)
	text = dedupeRepeatedLineBlocks(text)
	text = dedupeParagraphs(text)
	text = normalizeNumberedListPrefixes(text)
	text = canonicalizeDecisionMarkdown(text)
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

func stripCodeFormatting(text string) string {
	if text == "" {
		return ""
	}
	text = fenceBlockPattern.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "```", "")
	text = inlineCodePattern.ReplaceAllString(text, "$1")
	return strings.TrimSpace(text)
}

func normalizeNumberedListPrefixes(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			lines[i] = ""
			continue
		}
		line = numberedListPrefixPattern.ReplaceAllString(line, "")
		lines[i] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func canonicalizeDecisionMarkdown(text string) string {
	sections := parseDecisionSections(normalizeInlineDecisionHeadings(text))
	if len(sections) == 0 {
		return strings.TrimSpace(text)
	}

	order := []struct {
		key   string
		title string
		quote bool
	}{
		{key: "conclusion", title: "结论", quote: true},
		{key: "reason", title: "理由"},
		{key: "trigger", title: "触发与风控"},
		{key: "invalid", title: "失效条件"},
	}

	blocks := make([]string, 0, len(order))
	for _, item := range order {
		body := cleanDecisionSectionBody(sections[item.key])
		if body == "" {
			continue
		}
		if item.quote {
			body = formatAsQuoteBlock(body)
		}
		blocks = append(blocks, "## "+item.title+"\n"+body)
	}
	if len(blocks) == 0 {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func cleanDecisionSectionBody(text string) string {
	text = normalizeNumberedListPrefixes(stripCodeFormatting(text))
	if text == "" {
		return ""
	}
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if matches := decisionSectionLinePattern.FindStringSubmatch(line); len(matches) == 3 {
			line = strings.TrimSpace(matches[2])
		}
		line = strings.TrimPrefix(line, ">")
		line = strings.TrimSpace(numberedListPrefixPattern.ReplaceAllString(line, ""))
		if line == "" {
			continue
		}
		key := normalizeParagraphForCompare(line)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func formatAsQuoteBlock(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	quoted := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, ">")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		quoted = append(quoted, "> "+line)
	}
	return strings.Join(quoted, "\n")
}

func normalizeInlineDecisionHeadings(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = inlineDecisionHeadingPattern.ReplaceAllString(normalized, "$1\n$2")
	normalized = inlineBracketHeadingPattern.ReplaceAllString(normalized, "$1\n$2")
	normalized = inlineColonHeadingPattern.ReplaceAllString(normalized, "$1\n$2")
	normalized = inlineDecisionHeadingLoosePattern.ReplaceAllString(normalized, "$1\n$2")
	normalized = inlineBracketHeadingLoosePattern.ReplaceAllString(normalized, "$1\n$2")
	return normalized
}

type actionTone uint8

const (
	actionToneUnknown actionTone = iota
	actionToneConservative
	actionToneAggressive
)

func validateAgentOutputConsistency(content string) error {
	sections := parseDecisionSections(content)
	conclusion := strings.TrimSpace(sections["conclusion"])
	if conclusion == "" {
		return nil
	}
	conclusionTone := inferActionTone(conclusion)
	if conclusionTone == actionToneUnknown {
		return nil
	}

	body := strings.TrimSpace(strings.Join([]string{sections["reason"], sections["trigger"]}, "\n"))
	if body == "" {
		return nil
	}

	switch conclusionTone {
	case actionToneConservative:
		if hasUnconditionalAction(body, actionToneAggressive) {
			return fmt.Errorf("%w: 结论偏保守但后文出现无条件买入/加仓指令", ErrInconsistentReply)
		}
	case actionToneAggressive:
		if hasUnconditionalAction(body, actionToneConservative) {
			return fmt.Errorf("%w: 结论偏积极但后文出现无条件观望/减仓指令", ErrInconsistentReply)
		}
	}
	return nil
}

func parseDecisionSections(content string) map[string]string {
	sections := make(map[string]string, 4)
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	current := ""
	var buf strings.Builder
	flush := func() {
		if current == "" {
			return
		}
		part := strings.TrimSpace(buf.String())
		buf.Reset()
		if part == "" {
			return
		}
		if prev := sections[current]; prev != "" {
			prevNorm := normalizeParagraphForCompare(prev)
			partNorm := normalizeParagraphForCompare(part)
			if partNorm == "" || partNorm == prevNorm || strings.Contains(prevNorm, partNorm) {
				return
			}
			sections[current] = strings.TrimSpace(prev + "\n" + part)
		} else {
			sections[current] = part
		}
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		matches := decisionSectionLinePattern.FindStringSubmatch(line)
		if len(matches) == 3 {
			flush()
			current = normalizeDecisionSectionName(matches[1])
			if current == "" {
				continue
			}
			inline := strings.TrimSpace(matches[2])
			if inline != "" {
				buf.WriteString(inline)
			}
			continue
		}
		if current == "" {
			continue
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			if buf.Len() > 0 {
				buf.WriteString("\n")
			}
			continue
		}
		if buf.Len() > 0 && !strings.HasSuffix(buf.String(), "\n") {
			buf.WriteString("\n")
		}
		buf.WriteString(trimmed)
	}
	flush()
	return sections
}

func normalizeDecisionSectionName(name string) string {
	switch strings.TrimSpace(name) {
	case "结论", "综合结论":
		return "conclusion"
	case "理由", "共识与分歧":
		return "reason"
	case "触发与风控", "操作与风控", "触发", "风控", "风险与止损", "操作建议", "操作策略":
		return "trigger"
	case "失效条件", "失效":
		return "invalid"
	default:
		return ""
	}
}

func inferActionTone(text string) actionTone {
	content := strings.TrimSpace(text)
	if content == "" {
		return actionToneUnknown
	}

	conservativeScore := 0
	aggressiveScore := 0

	for _, kw := range conservativeActionKeywords {
		if strings.Contains(content, kw) {
			conservativeScore++
		}
	}
	for _, kw := range aggressiveActionKeywords {
		if strings.Contains(content, kw) {
			aggressiveScore++
		}
	}
	for _, kw := range aggressiveNegativePhrases {
		if strings.Contains(content, kw) {
			conservativeScore += 2
		}
	}

	if conservativeScore == 0 && aggressiveScore == 0 {
		return actionToneUnknown
	}
	if conservativeScore >= aggressiveScore+1 {
		return actionToneConservative
	}
	if aggressiveScore >= conservativeScore+1 {
		return actionToneAggressive
	}
	return actionToneUnknown
}

func hasUnconditionalAction(text string, target actionTone) bool {
	for _, sentence := range splitTextSentences(text) {
		s := strings.TrimSpace(sentence)
		if s == "" {
			continue
		}
		if target == actionToneAggressive && containsAny(s, aggressiveNegativePhrases) {
			continue
		}
		if containsAny(s, conditionalSentenceMarkers) {
			continue
		}
		if inferActionTone(s) == target {
			return true
		}
	}
	return false
}

func splitTextSentences(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		switch r {
		case '\n', '\r', '。', '！', '!', '？', '?', '；', ';':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if kw != "" && strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func stripReferenceSection(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	cut := -1
	keys := []string{"**参考依据**", "参考依据", "数据依据"}
	for _, key := range keys {
		idx := strings.Index(trimmed, key)
		if idx >= 0 && (cut < 0 || idx < cut) {
			cut = idx
		}
	}
	if cut >= 0 {
		return strings.TrimSpace(trimmed[:cut])
	}
	return trimmed
}

func dedupeParagraphs(text string) string {
	paras := strings.Split(text, "\n\n")
	out := make([]string, 0, len(paras))
	seen := make(map[string]struct{})
	for _, para := range paras {
		p := strings.TrimSpace(para)
		if p == "" {
			continue
		}
		norm := normalizeParagraphForCompare(p)
		if norm == "" {
			continue
		}
		// 仅对较长段落做全局去重，短标题保留。
		if len(norm) >= 24 {
			if _, ok := seen[norm]; ok {
				continue
			}
			seen[norm] = struct{}{}
		}
		out = append(out, p)
	}
	return strings.TrimSpace(strings.Join(out, "\n\n"))
}

func normalizeParagraphForCompare(input string) string {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return ""
	}
	s = agentRefMarkerPattern.ReplaceAllString(s, "")
	replacer := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\t", "",
		"\r", "",
		"*", "",
		"#", "",
		"-", "",
		"_", "",
		"`", "",
		">", "",
	)
	s = replacer.Replace(s)
	return strings.TrimSpace(s)
}

func dedupeRepeatedTail(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return trimmed
	}
	markers := []string{"【结论】", "[结论]", "结论：", "结论:", "**结论**", "## 结论", "### 结论", "##结论", "###结论"}
	for _, marker := range markers {
		first := strings.Index(trimmed, marker)
		if first < 0 {
			continue
		}
		secondRel := strings.Index(trimmed[first+len(marker):], marker)
		if secondRel < 0 {
			continue
		}
		second := first + len(marker) + secondRel
		headNorm := normalizeParagraphForCompare(trimmed[:second])
		tailNorm := normalizeParagraphForCompare(trimmed[second:])
		if len(tailNorm) < 64 {
			continue
		}
		probeLen := 180
		if len(tailNorm) < probeLen {
			probeLen = len(tailNorm)
		}
		if probeLen >= 48 && strings.Contains(headNorm, tailNorm[:probeLen]) {
			return strings.TrimSpace(trimmed[:second])
		}
	}
	return trimmed
}

func dedupeRepeatedLineBlocks(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) < 8 {
		return text
	}

	for window := minInt(18, len(lines)/2); window >= 4; window-- {
		changed := false
		for i := 0; i+window*2 <= len(lines); i++ {
			left := normalizeParagraphForCompare(strings.Join(lines[i:i+window], ""))
			right := normalizeParagraphForCompare(strings.Join(lines[i+window:i+window*2], ""))
			if left == "" || right == "" {
				continue
			}
			if left == right {
				lines = append(lines[:i+window], lines[i+window*2:]...)
				changed = true
				break
			}
		}
		if changed {
			window = minInt(18, len(lines)/2+1)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func marshalLogJSON(value any) string {
	if value == nil {
		return "null"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	return string(raw)
}

func normalizeToolStockSymbol(raw string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return ""
	}
	if strings.HasPrefix(candidate, "s_") {
		candidate = strings.TrimPrefix(candidate, "s_")
	}
	if match := stockSymbolPattern.FindString(candidate); match != "" {
		return strings.ToLower(match)
	}
	if digits := sixDigitSymbolPattern.FindString(candidate); digits != "" {
		switch digits[0] {
		case '6':
			return "sh" + digits
		case '0', '3':
			return "sz" + digits
		case '4', '8':
			return "bj" + digits
		}
	}
	return ""
}

func cloneToolArgs(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(raw))
	for k, v := range raw {
		cloned[k] = v
	}
	return cloned
}

func getToolArgInt(args map[string]any, key string) (int, bool) {
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(text, "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func ensurePositiveToolArg(args map[string]any, key string, fallback int) {
	if v, ok := getToolArgInt(args, key); ok && v > 0 {
		return
	}
	args[key] = fallback
}

func hasNonEmptyToolStringArg(args map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		if strings.TrimSpace(fmt.Sprintf("%v", value)) != "" {
			return true
		}
	}
	return false
}

func hasCodesToolArg(args map[string]any) bool {
	value, ok := args["codes"]
	if !ok || value == nil {
		return false
	}
	switch codes := value.(type) {
	case []string:
		for _, code := range codes {
			if normalizeToolStockSymbol(code) != "" {
				return true
			}
		}
	case []any:
		for _, code := range codes {
			if normalizeToolStockSymbol(fmt.Sprintf("%v", code)) != "" {
				return true
			}
		}
	default:
		return normalizeToolStockSymbol(fmt.Sprintf("%v", value)) != ""
	}
	return false
}

func hasStockToolCodeArg(args map[string]any) bool {
	if hasCodesToolArg(args) {
		return true
	}
	return hasNonEmptyToolStringArg(args, "code", "symbol", "stockCode", "ticker", "securityCode", "secuCode")
}

func ensureStockToolCodeArg(args map[string]any, symbol string) {
	if symbol == "" || hasStockToolCodeArg(args) {
		return
	}
	args["code"] = symbol
}

func defaultIndexByStockSymbol(symbol string) string {
	switch {
	case strings.HasPrefix(symbol, "sh"):
		return "sh000001"
	case strings.HasPrefix(symbol, "sz"), strings.HasPrefix(symbol, "bj"):
		return "sz399001"
	default:
		return "sh000001"
	}
}

func hydrateToolCallArgs(callName string, rawArgs map[string]any, stock *models.Stock) map[string]any {
	args := cloneToolArgs(rawArgs)
	callName = strings.TrimSpace(callName)
	symbol := ""
	if stock != nil {
		symbol = normalizeToolStockSymbol(stock.Symbol)
	}

	if strings.HasPrefix(callName, "get_f10_") || callName == "get_core_data_pack" {
		ensureStockToolCodeArg(args, symbol)
	}

	switch callName {
	case "get_kline_data":
		ensureStockToolCodeArg(args, symbol)
		if !hasNonEmptyToolStringArg(args, "period") {
			args["period"] = "1d"
		}
		ensurePositiveToolArg(args, "days", 30)
	case "get_stock_realtime":
		if symbol != "" && !hasStockToolCodeArg(args) {
			args["codes"] = []string{symbol}
		}
	case "get_orderbook":
		ensureStockToolCodeArg(args, symbol)
	case "get_stock_announcements":
		ensureStockToolCodeArg(args, symbol)
		ensurePositiveToolArg(args, "page", 1)
		ensurePositiveToolArg(args, "pageSize", 10)
	case "get_index_fund_flow":
		if !hasNonEmptyToolStringArg(args, "code", "symbol", "stockCode", "indexCode", "secid", "ticker", "securityCode", "secuCode") {
			args["code"] = defaultIndexByStockSymbol(symbol)
		}
		if !hasNonEmptyToolStringArg(args, "interval") {
			args["interval"] = "1"
		}
	case "get_research_report":
		ensureStockToolCodeArg(args, symbol)
		ensurePositiveToolArg(args, "pageNo", 1)
		ensurePositiveToolArg(args, "pageSize", 10)
	case "get_longhubang":
		ensurePositiveToolArg(args, "page_size", 20)
		ensurePositiveToolArg(args, "page_number", 1)
	case "get_board_fund_flow":
		ensurePositiveToolArg(args, "page", 1)
		ensurePositiveToolArg(args, "pageSize", 20)
	case "get_stock_moves":
		ensurePositiveToolArg(args, "page", 1)
		ensurePositiveToolArg(args, "pageSize", 30)
	}

	return args
}

func (s *Service) logAgentRequestPayload(cfg *models.AgentConfig, sessionID string, instruction string, userInput string) {
	if !s.verboseAgentIO {
		return
	}
	log.Info("AgentIO request begin agent=%s session=%s", cfg.ID, sessionID)
	log.Info("AgentIO request instruction agent=%s session=%s content=\n%s", cfg.ID, sessionID, instruction)
	log.Info("AgentIO request user_input agent=%s session=%s content=\n%s", cfg.ID, sessionID, userInput)
	log.Info("AgentIO request end agent=%s session=%s", cfg.ID, sessionID)
}

func (s *Service) logAgentToolCall(cfg *models.AgentConfig, sessionID string, stock *models.Stock, call *genai.FunctionCall) {
	if !s.verboseAgentIO || call == nil {
		return
	}
	effectiveArgs := hydrateToolCallArgs(call.Name, call.Args, stock)
	if reflect.DeepEqual(effectiveArgs, call.Args) {
		log.Info(
			"AgentIO tool_call agent=%s session=%s id=%s name=%s args=%s",
			cfg.ID,
			sessionID,
			call.ID,
			call.Name,
			marshalLogJSON(effectiveArgs),
		)
		return
	}
	log.Info(
		"AgentIO tool_call agent=%s session=%s id=%s name=%s args=%s rawArgs=%s",
		cfg.ID,
		sessionID,
		call.ID,
		call.Name,
		marshalLogJSON(effectiveArgs),
		marshalLogJSON(call.Args),
	)
}

func (s *Service) logAgentToolResult(cfg *models.AgentConfig, sessionID string, result *genai.FunctionResponse) {
	if !s.verboseAgentIO || result == nil {
		return
	}
	log.Info(
		"AgentIO tool_result agent=%s session=%s id=%s name=%s output=%s",
		cfg.ID,
		sessionID,
		result.ID,
		result.Name,
		marshalLogJSON(result.Response),
	)
}

func (s *Service) logAgentFinalResponse(cfg *models.AgentConfig, sessionID string, content string) {
	if !s.verboseAgentIO {
		return
	}
	log.Info("AgentIO response begin agent=%s session=%s", cfg.ID, sessionID)
	log.Info("AgentIO response content agent=%s session=%s content=\n%s", cfg.ID, sessionID, content)
	log.Info("AgentIO response end agent=%s session=%s", cfg.ID, sessionID)
}

func contextDeadlineLeft(ctx context.Context) string {
	if ctx == nil {
		return "n/a"
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return "none"
	}
	left := time.Until(deadline)
	if left < 0 {
		left = 0
	}
	return left.Round(time.Millisecond).String()
}

func resolveConfiguredTimeout(cfg *models.AIConfig, fallback time.Duration) time.Duration {
	if cfg == nil || cfg.Timeout <= 0 {
		return fallback
	}
	d := time.Duration(cfg.Timeout) * time.Second
	if d < MinConfigTimeout {
		return MinConfigTimeout
	}
	if d > MaxConfigTimeout {
		return MaxConfigTimeout
	}
	return d
}

func (s *Service) resolveAgentTimeout(cfg *models.AIConfig) time.Duration {
	return resolveConfiguredTimeout(cfg, AgentTimeout)
}

// resolveAgentAttemptTimeout 计算单次 attempt 的超时时间。
// 当启用 Responses API 时，主请求与 fallback 会共用同一个 context。
// 为避免 primary 耗尽预算导致 fallback 立即 context canceled，预留一段缓冲时间。
func (s *Service) resolveAgentAttemptTimeout(cfg *models.AIConfig) time.Duration {
	timeout := s.resolveAgentTimeout(cfg)
	if cfg == nil {
		return timeout
	}
	if cfg.Provider != models.AIProviderOpenAI || !cfg.UseResponses {
		return timeout
	}

	grace := ResponsesFallbackGrace
	if timeout+grace > MaxConfigTimeout {
		grace = MaxConfigTimeout - timeout
		if grace < 0 {
			grace = 0
		}
	}
	return timeout + grace
}

func (s *Service) resolveModeratorTimeout(defaultCfg *models.AIConfig) time.Duration {
	cfg := defaultCfg
	if s.moderatorAIConfig != nil {
		cfg = s.moderatorAIConfig
	}
	return resolveConfiguredTimeout(cfg, ModeratorTimeout)
}

func (s *Service) resolveSmartMeetingTimeout(defaultCfg *models.AIConfig, req ChatRequest) time.Duration {
	maxAgentTimeout := s.resolveAgentAttemptTimeout(defaultCfg)
	for _, agentCfg := range req.AllAgents {
		cfg := defaultCfg
		if s.aiConfigResolver != nil && agentCfg.AIConfigID != "" {
			if resolved := s.aiConfigResolver(agentCfg.AIConfigID); resolved != nil {
				cfg = resolved
			}
		}
		agentTimeout := s.resolveAgentAttemptTimeout(cfg)
		if agentTimeout > maxAgentTimeout {
			maxAgentTimeout = agentTimeout
		}
	}

	agentCount := len(req.AllAgents)
	if agentCount < 1 {
		agentCount = 1
	}
	moderatorTimeout := s.resolveModeratorTimeout(defaultCfg)
	estimated := time.Duration(agentCount)*maxAgentTimeout + moderatorTimeout*2 + 30*time.Second
	if estimated < MeetingTimeout {
		return MeetingTimeout
	}
	if estimated > MaxConfigTimeout {
		return MaxConfigTimeout
	}
	return estimated
}

func (s *Service) resolveParallelMeetingTimeout(defaultCfg *models.AIConfig, req ChatRequest) time.Duration {
	maxAgentTimeout := s.resolveAgentAttemptTimeout(defaultCfg)
	for _, agentCfg := range req.Agents {
		cfg := defaultCfg
		if s.aiConfigResolver != nil && agentCfg.AIConfigID != "" {
			if resolved := s.aiConfigResolver(agentCfg.AIConfigID); resolved != nil {
				cfg = resolved
			}
		}
		agentTimeout := s.resolveAgentAttemptTimeout(cfg)
		if agentTimeout > maxAgentTimeout {
			maxAgentTimeout = agentTimeout
		}
	}

	estimated := maxAgentTimeout + 60*time.Second
	if estimated < MeetingTimeout {
		return MeetingTimeout
	}
	if estimated > MaxConfigTimeout {
		return MaxConfigTimeout
	}
	return estimated
}

func buildAgentUserQuery(stock *models.Stock, query string, position *models.StockPosition) string {
	query = strings.TrimSpace(query)
	if stock == nil {
		return query
	}

	symbol := strings.TrimSpace(stock.Symbol)
	name := strings.TrimSpace(stock.Name)
	if symbol == "" && name == "" {
		return query
	}

	identity := symbol
	if name != "" && symbol != "" {
		identity = fmt.Sprintf("%s (%s)", name, symbol)
	} else if name != "" {
		identity = name
	}

	var sb strings.Builder
	sb.WriteString("【当前讨论股票】")
	sb.WriteString(identity)
	sb.WriteString("\n【回答约束】请仅围绕该股票回答，不要要求用户再次提供股票代码/名称；若个别数据缺失，请直接说明缺失项并基于现有数据给出判断。")
	if position != nil && position.Shares > 0 {
		costAmount := float64(position.Shares) * position.CostPrice
		marketValue := 0.0
		pnlValue := 0.0
		pnlRatio := 0.0
		if stock != nil && stock.Price > 0 {
			marketValue = float64(position.Shares) * stock.Price
			pnlValue = marketValue - costAmount
			if costAmount > 0 {
				pnlRatio = pnlValue / costAmount * 100
			}
		}
		sb.WriteString(fmt.Sprintf("\n【用户持仓】持有 %d 股，成本价 %.2f", position.Shares, position.CostPrice))
		if stock != nil && stock.Price > 0 {
			sb.WriteString(fmt.Sprintf("，当前价 %.2f，浮盈亏 %.2f（%.2f%%）", stock.Price, pnlValue, pnlRatio))
		}
		sb.WriteString("。请结合持仓成本与风险承受给出建议。")
	}
	sb.WriteString("\n【用户问题】")
	sb.WriteString(query)
	return sb.String()
}

// SendMessage 发送会议消息，生成多专家回复（并行执行）
func (s *Service) SendMessage(ctx context.Context, aiConfig *models.AIConfig, req ChatRequest) ([]ChatResponse, error) {
	llm, err := s.modelFactory.CreateModel(ctx, aiConfig)
	if err != nil {
		log.Error("CreateModel error: %v", err)
		return nil, err
	}
	log.Info("model created successfully")

	return s.runAgentsParallel(ctx, llm, aiConfig, req)
}

// RunSmartMeeting 智能会议模式（小韭菜编排）
// 专家按顺序串行发言，后一个专家可以参考前面的发言内容
func (s *Service) RunSmartMeeting(ctx context.Context, aiConfig *models.AIConfig, req ChatRequest) ([]ChatResponse, error) {
	return s.RunSmartMeetingWithCallback(ctx, aiConfig, req, nil, nil)
}

// RunSmartMeetingWithCallback 智能会议模式（带实时回调）
// respCallback 在每个发言完成后调用
// progressCallback 在工具调用、流式输出等细粒度事件时调用
func (s *Service) RunSmartMeetingWithCallback(ctx context.Context, aiConfig *models.AIConfig, req ChatRequest, respCallback ResponseCallback, progressCallback ProgressCallback) ([]ChatResponse, error) {
	if aiConfig == nil {
		return nil, ErrNoAIConfig
	}
	if len(req.AllAgents) == 0 {
		return nil, ErrNoAgents
	}

	// 设置整个会议的超时上下文（可随模型 timeout 动态放宽）
	meetingTimeout := s.resolveSmartMeetingTimeout(aiConfig, req)
	meetingCtx, meetingCancel := context.WithTimeout(ctx, meetingTimeout)
	defer meetingCancel()
	log.Info("meeting timeout resolved: %s", meetingTimeout)

	// 创建模型（带超时）
	modelCtx, modelCancel := context.WithTimeout(meetingCtx, ModelCreationTimeout)
	llm, err := s.modelFactory.CreateModel(modelCtx, aiConfig)
	modelCancel()
	if err != nil {
		return nil, fmt.Errorf("create model error: %w", err)
	}
	llmPool := newLLMPool(aiConfig, llm, s.modelFactory.CreateModel)

	var responses []ChatResponse

	// 创建 Moderator LLM（优先使用独立配置）
	var moderatorLLM model.LLM
	if s.moderatorAIConfig != nil {
		moderatorLLM, err = llmPool.get(meetingCtx, s.moderatorAIConfig)
		if err != nil {
			log.Warn("create moderator LLM error, fallback to default: %v", err)
			moderatorLLM = llm
		} else {
			log.Debug("using dedicated moderator LLM: %s", s.moderatorAIConfig.ModelName)
		}
	} else {
		moderatorLLM = llm
	}
	moderator := NewModerator(moderatorLLM)

	// 设置 LLM 到记忆管理器（启用摘要功能）
	if s.memoryManager != nil {
		// 优先使用配置的记忆 LLM，否则使用会议 LLM
		if s.memoryAIConfig != nil {
			memoryLLM, err := llmPool.get(meetingCtx, s.memoryAIConfig)
			if err == nil {
				s.memoryManager.SetLLM(memoryLLM)
				log.Debug("using dedicated memory LLM: %s", s.memoryAIConfig.ModelName)
			} else {
				log.Warn("create memory LLM error, fallback to meeting LLM: %v", err)
				s.memoryManager.SetLLM(llm)
			}
		} else {
			s.memoryManager.SetLLM(llm)
		}
	}

	// 加载股票记忆（如果启用了记忆管理）
	var stockMemory *memory.StockMemory
	var memoryContext string
	if s.memoryManager != nil {
		stockMemory, _ = s.memoryManager.GetOrCreate(req.Stock.Symbol, req.Stock.Name)
		memoryContext = s.memoryManager.BuildContext(stockMemory, req.Query)
		if memoryContext != "" {
			log.Debug("loaded memory context for %s, len: %d", req.Stock.Symbol, len(memoryContext))
		}
	}

	log.Info("stock: %s, query: %s, agents: %d", req.Stock.Symbol, req.Query, len(req.AllAgents))

	// 第0轮：小韭菜分析意图并选择专家（带超时）
	if progressCallback != nil {
		progressCallback(ProgressEvent{
			Type:      "agent_start",
			AgentID:   "moderator",
			AgentName: "小韭菜",
			Detail:    "分析问题意图",
		})
	}

	moderatorTimeout := s.resolveModeratorTimeout(aiConfig)
	moderatorCtx, moderatorCancel := context.WithTimeout(meetingCtx, moderatorTimeout)
	decision, err := s.analyzeWithRetry(moderatorCtx, moderator, &req.Stock, req.Query, req.AllAgents)
	moderatorCancel()
	log.Debug("moderator analyze timeout resolved: %s", moderatorTimeout)

	if err != nil {
		if progressCallback != nil {
			doneDetail := "error"
			if errors.Is(err, context.DeadlineExceeded) {
				doneDetail = "timeout"
			} else {
				doneDetail = "error: " + normalizeLogText(err.Error(), 80)
			}
			progressCallback(ProgressEvent{
				Type:      "agent_done",
				AgentID:   "moderator",
				AgentName: "小韭菜",
				Detail:    doneDetail,
			})
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: 小韭菜分析超时", ErrModeratorTimeout)
		}
		return nil, fmt.Errorf("moderator analyze error: %w", err)
	}

	if progressCallback != nil {
		progressCallback(ProgressEvent{
			Type:      "agent_done",
			AgentID:   "moderator",
			AgentName: "小韭菜",
		})
	}

	log.Debug("decision: selected=%v, topic=%s", decision.Selected, decision.Topic)

	// 添加开场白并立即回调
	openingResp := ChatResponse{
		AgentID:   "moderator",
		AgentName: "小韭菜",
		Role:      "会议主持",
		Content:   decision.Opening,
		Round:     0,
		MsgType:   "opening",
	}
	responses = append(responses, openingResp)
	if respCallback != nil {
		respCallback(openingResp)
	}

	// 筛选被选中的专家（按小韭菜选择的顺序）
	selectedAgents := s.filterAgentsOrdered(req.AllAgents, decision.Selected)
	if len(selectedAgents) == 0 {
		log.Warn("moderator selected invalid agents: %v, fallback to default experts", decision.Selected)
		selectedAgents = s.fallbackAgents(req.AllAgents, 2)
		if len(selectedAgents) == 0 {
			return responses, nil
		}
	}
	selectedAgents = s.enforceSelectionDiversity(req.AllAgents, selectedAgents, req.Query, req.Stock.Symbol)
	log.Debug("selected agents after diversity balancing: %v", extractAgentIDs(selectedAgents))

	// 第1轮：专家串行发言，后一个参考前面的内容
	var history []DiscussionEntry
	basePreviousContext := strings.TrimSpace(req.ReplyContent)

	for i, agentCfg := range selectedAgents {
		// 检查会议是否已超时
		select {
		case <-meetingCtx.Done():
			log.Warn("meeting timeout, got %d responses", len(responses))
			return responses, ErrMeetingTimeout
		default:
		}

		log.Debug("agent %d/%d: %s starting", i+1, len(selectedAgents), agentCfg.Name)

		// 获取该专家的 AI 配置
		agentAIConfig := aiConfig // 默认使用传入的配置
		if s.aiConfigResolver != nil && agentCfg.AIConfigID != "" {
			if resolved := s.aiConfigResolver(agentCfg.AIConfigID); resolved != nil {
				agentAIConfig = resolved
				log.Debug("agent %s using custom AI: %s", agentCfg.ID, resolved.ModelName)
			}
		}
		aiModel := ""
		if agentAIConfig != nil {
			aiModel = agentAIConfig.ModelName
		}
		log.Debug(
			"agent %s config ready, model=%s tools=%d mcpServers=%d",
			agentCfg.ID,
			aiModel,
			len(agentCfg.Tools),
			len(agentCfg.MCPServers),
		)

		// 为该专家创建 LLM
		agentLLM, err := llmPool.get(meetingCtx, agentAIConfig)
		if err != nil {
			log.Error("create agent LLM error, agent=%s model=%s err=%s", agentCfg.ID, aiModel, normalizeLogText(err.Error(), 320))
			continue
		}
		builder := s.createBuilder(agentLLM, agentAIConfig)

		// 发送专家开始事件
		if progressCallback != nil {
			progressCallback(ProgressEvent{
				Type:      "agent_start",
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
				Detail:    agentCfg.Role,
			})
		}

		// 构建前面专家发言的上下文
		previousContext := s.buildPreviousContext(history)
		if basePreviousContext != "" {
			if previousContext != "" {
				previousContext = basePreviousContext + "\n\n" + previousContext
			} else {
				previousContext = basePreviousContext
			}
		}
		// 合并记忆上下文
		if memoryContext != "" {
			previousContext = memoryContext + "\n" + previousContext
		}

		// 运行单个专家（带超时控制）
		configuredTimeout := s.resolveAgentTimeout(agentAIConfig)
		attemptTimeout := s.resolveAgentAttemptTimeout(agentAIConfig)
		content, err := s.runSingleAgentWithHistory(meetingCtx, attemptTimeout, builder, &agentCfg, &req.Stock, req.Query, previousContext, req.CoreContext, req.IntentContext, progressCallback, req.Position)
		log.Debug("agent %s timeout resolved: configured=%s attempt=%s", agentCfg.ID, configuredTimeout, attemptTimeout)

		if err != nil {
			doneDetail := "error"
			if errors.Is(err, context.DeadlineExceeded) {
				doneDetail = "timeout"
			} else {
				doneDetail = "error: " + normalizeLogText(err.Error(), 80)
			}
			// 发送专家完成事件（即使失败）
			if progressCallback != nil {
				progressCallback(ProgressEvent{
					Type:      "agent_done",
					AgentID:   agentCfg.ID,
					AgentName: agentCfg.Name,
					Detail:    doneDetail,
				})
			}
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warn("agent %s timeout", agentCfg.ID)
			} else {
				log.Error("agent %s error: %v", agentCfg.ID, err)
			}
			failResp := ChatResponse{
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
				Role:      agentCfg.Role,
				Content:   buildAgentFailureContent(err),
				Round:     1,
				MsgType:   "opinion",
			}
			responses = append(responses, failResp)
			if respCallback != nil {
				respCallback(failResp)
			}
			continue
		}

		if strings.TrimSpace(content) == "" {
			log.Warn("agent %s skipped empty content after retries", agentCfg.ID)
			failResp := ChatResponse{
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
				Role:      agentCfg.Role,
				Content:   buildAgentFailureContent(ErrEmptyAgentReply),
				Round:     1,
				MsgType:   "opinion",
			}
			responses = append(responses, failResp)
			if respCallback != nil {
				respCallback(failResp)
			}
			continue
		}

		// 发送专家完成事件
		if progressCallback != nil {
			progressCallback(ProgressEvent{
				Type:      "agent_done",
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
			})
		}

		// 添加到响应并立即回调
		resp := ChatResponse{
			AgentID:   agentCfg.ID,
			AgentName: agentCfg.Name,
			Role:      agentCfg.Role,
			Content:   content,
			Round:     1,
			MsgType:   "opinion",
		}
		responses = append(responses, resp)
		if respCallback != nil {
			respCallback(resp)
		}

		// 记录到历史
		history = append(history, DiscussionEntry{
			Round:     1,
			AgentID:   agentCfg.ID,
			AgentName: agentCfg.Name,
			Role:      agentCfg.Role,
			Content:   content,
		})

		log.Debug("agent %s done, content len: %d", agentCfg.ID, len(content))
	}

	// 二次补充数据与校正轮
	supplementContext := ""
	if s.supplementBuilder != nil && len(history) > 0 {
		supplementContext = s.supplementBuilder(req.Stock, req.Query, history)
	}

	if s.enableSecondRound && supplementContext != "" && len(selectedAgents) > 0 {
		log.Debug("supplement context ready, start revision round")
		var revisionHistory []DiscussionEntry
		revisionQuery := fmt.Sprintf("%s\n\n请基于补充数据对上一轮观点进行修正/补充，指出需要调整的地方。", req.Query)

		for i, agentCfg := range selectedAgents {
			select {
			case <-meetingCtx.Done():
				log.Warn("meeting timeout during revision, got %d responses", len(responses))
				return responses, ErrMeetingTimeout
			default:
			}

			log.Debug("revision agent %d/%d: %s starting", i+1, len(selectedAgents), agentCfg.Name)

			agentAIConfig := aiConfig
			if s.aiConfigResolver != nil && agentCfg.AIConfigID != "" {
				if resolved := s.aiConfigResolver(agentCfg.AIConfigID); resolved != nil {
					agentAIConfig = resolved
					log.Debug("agent %s using custom AI: %s", agentCfg.ID, resolved.ModelName)
				}
			}
			aiModel := ""
			if agentAIConfig != nil {
				aiModel = agentAIConfig.ModelName
			}
			log.Debug(
				"revision agent %s config ready, model=%s tools=%d mcpServers=%d",
				agentCfg.ID,
				aiModel,
				len(agentCfg.Tools),
				len(agentCfg.MCPServers),
			)

			agentLLM, err := llmPool.get(meetingCtx, agentAIConfig)
			if err != nil {
				log.Error("create agent LLM error, agent=%s model=%s err=%s", agentCfg.ID, aiModel, normalizeLogText(err.Error(), 320))
				continue
			}
			builder := s.createBuilder(agentLLM, agentAIConfig)

			if progressCallback != nil {
				progressCallback(ProgressEvent{
					Type:      "agent_start",
					AgentID:   agentCfg.ID,
					AgentName: agentCfg.Name,
					Detail:    "二次校正",
				})
			}

			combinedHistory := append(append([]DiscussionEntry{}, history...), revisionHistory...)
			previousContext := s.buildPreviousContext(combinedHistory)
			if basePreviousContext != "" {
				if previousContext != "" {
					previousContext = basePreviousContext + "\n\n" + previousContext
				} else {
					previousContext = basePreviousContext
				}
			}
			if memoryContext != "" {
				previousContext = memoryContext + "\n" + previousContext
			}

			intentContext := req.IntentContext
			if intentContext != "" {
				intentContext = intentContext + "\n" + supplementContext
			} else {
				intentContext = supplementContext
			}

			configuredTimeout := s.resolveAgentTimeout(agentAIConfig)
			attemptTimeout := s.resolveAgentAttemptTimeout(agentAIConfig)
			content, err := s.runSingleAgentWithHistory(meetingCtx, attemptTimeout, builder, &agentCfg, &req.Stock, revisionQuery, previousContext, req.CoreContext, intentContext, progressCallback, req.Position)
			log.Debug("revision agent %s timeout resolved: configured=%s attempt=%s", agentCfg.ID, configuredTimeout, attemptTimeout)

			if progressCallback != nil {
				doneDetail := ""
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						doneDetail = "timeout"
					} else {
						doneDetail = "error: " + normalizeLogText(err.Error(), 80)
					}
				}
				progressCallback(ProgressEvent{
					Type:      "agent_done",
					AgentID:   agentCfg.ID,
					AgentName: agentCfg.Name,
					Detail:    doneDetail,
				})
			}

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					log.Warn("agent %s timeout (revision)", agentCfg.ID)
				} else {
					log.Error("agent %s error (revision): %v", agentCfg.ID, err)
				}
				failResp := ChatResponse{
					AgentID:   agentCfg.ID,
					AgentName: agentCfg.Name,
					Role:      agentCfg.Role,
					Content:   buildAgentFailureContent(err),
					Round:     2,
					MsgType:   "opinion",
				}
				responses = append(responses, failResp)
				if respCallback != nil {
					respCallback(failResp)
				}
				continue
			}
			if strings.TrimSpace(content) == "" {
				log.Warn("agent %s skipped empty content after retries (revision)", agentCfg.ID)
				failResp := ChatResponse{
					AgentID:   agentCfg.ID,
					AgentName: agentCfg.Name,
					Role:      agentCfg.Role,
					Content:   buildAgentFailureContent(ErrEmptyAgentReply),
					Round:     2,
					MsgType:   "opinion",
				}
				responses = append(responses, failResp)
				if respCallback != nil {
					respCallback(failResp)
				}
				continue
			}

			resp := ChatResponse{
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
				Role:      agentCfg.Role,
				Content:   content,
				Round:     2,
				MsgType:   "opinion",
			}
			responses = append(responses, resp)
			if respCallback != nil {
				respCallback(resp)
			}

			revisionHistory = append(revisionHistory, DiscussionEntry{
				Round:     2,
				AgentID:   agentCfg.ID,
				AgentName: agentCfg.Name,
				Role:      agentCfg.Role,
				Content:   content,
			})
		}

		history = append(history, revisionHistory...)
	}

	// 最终轮：小韭菜总结（带超时）
	if progressCallback != nil {
		progressCallback(ProgressEvent{
			Type:      "agent_start",
			AgentID:   "moderator",
			AgentName: "小韭菜",
			Detail:    "总结讨论",
		})
	}

	summaryTimeout := s.resolveModeratorTimeout(aiConfig)
	summaryCtx, summaryCancel := context.WithTimeout(meetingCtx, summaryTimeout)
	extraContext := s.buildSummaryContext(req.CoreContext, req.IntentContext, supplementContext, &req.Stock, req.Position)
	summary, err := s.summarizeWithRetry(summaryCtx, moderator, &req.Stock, req.Query, history, extraContext)
	summaryCancel()
	log.Debug("moderator summary timeout resolved: %s", summaryTimeout)

	if progressCallback != nil {
		progressCallback(ProgressEvent{
			Type:      "agent_done",
			AgentID:   "moderator",
			AgentName: "小韭菜",
		})
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn("summary timeout, returning partial results")
		} else {
			log.Error("summary error: %v", err)
		}
		// 总结失败不影响返回已有结果
		return responses, nil
	}

	if summary != "" {
		summaryResp := ChatResponse{
			AgentID:   "moderator",
			AgentName: "小韭菜",
			Role:      "会议主持",
			Content:   summary,
			Round:     3,
			MsgType:   "summary",
		}
		responses = append(responses, summaryResp)
		if respCallback != nil {
			respCallback(summaryResp)
		}
	}

	// 保存记忆（如果启用了记忆管理）
	if s.memoryManager != nil && stockMemory != nil && summary != "" {
		// 异步保存记忆，不阻塞返回
		go func() {
			// 使用独立 context，因为会议 ctx 可能已取消
			bgCtx := context.Background()
			keyPoints := s.extractKeyPointsFromHistory(bgCtx, history)
			if err := s.memoryManager.AddRound(bgCtx, stockMemory, req.Query, summary, keyPoints); err != nil {
				log.Error("save memory error: %v", err)
			} else {
				log.Debug("saved memory for %s", req.Stock.Symbol)
			}
		}()
	}

	return responses, nil
}

// runAgentsParallel 并行运行多个 Agent（带超时控制）
func (s *Service) runAgentsParallel(ctx context.Context, defaultLLM model.LLM, defaultAIConfig *models.AIConfig, req ChatRequest) ([]ChatResponse, error) {
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		responses []ChatResponse
	)

	// 设置整体超时（可随模型 timeout 动态放宽）
	parallelTimeout := s.resolveParallelMeetingTimeout(defaultAIConfig, req)
	parallelCtx, cancel := context.WithTimeout(ctx, parallelTimeout)
	defer cancel()
	llmPool := newLLMPool(defaultAIConfig, defaultLLM, s.modelFactory.CreateModel)
	log.Info("parallel meeting timeout resolved: %s", parallelTimeout)

	log.Debug("running %d agents in parallel", len(req.Agents))

	for _, agentConfig := range req.Agents {
		wg.Add(1)
		go func(cfg models.AgentConfig) {
			defer wg.Done()

			// 获取该专家的 AI 配置
			agentAIConfig := defaultAIConfig
			if s.aiConfigResolver != nil && cfg.AIConfigID != "" {
				if resolved := s.aiConfigResolver(cfg.AIConfigID); resolved != nil {
					agentAIConfig = resolved
					log.Debug("agent %s using custom AI: %s", cfg.ID, resolved.ModelName)
				}
			}
			aiModel := ""
			if agentAIConfig != nil {
				aiModel = agentAIConfig.ModelName
			}
			log.Debug(
				"parallel agent %s config ready, model=%s tools=%d mcpServers=%d",
				cfg.ID,
				aiModel,
				len(cfg.Tools),
				len(cfg.MCPServers),
			)

			// 为该专家创建/复用 LLM
			agentLLM, err := llmPool.get(parallelCtx, agentAIConfig)
			if err != nil {
				log.Error("create agent LLM error, agent=%s model=%s err=%s", cfg.ID, aiModel, normalizeLogText(err.Error(), 320))
				return
			}
			builder := s.createBuilder(agentLLM, agentAIConfig)

			// 单个 Agent 超时控制（按 AI 配置动态）
			configuredTimeout := s.resolveAgentTimeout(agentAIConfig)
			attemptTimeout := s.resolveAgentAttemptTimeout(agentAIConfig)
			log.Debug("parallel agent %s timeout resolved: configured=%s attempt=%s", cfg.ID, configuredTimeout, attemptTimeout)

			content, err := s.runSingleAgentWithContext(parallelCtx, attemptTimeout, builder, &cfg, &req.Stock, req.Query, req.ReplyContent, req.CoreContext, req.IntentContext, req.Position)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					log.Warn("agent %s timeout", cfg.ID)
				} else {
					log.Error("agent %s error: %v", cfg.ID, err)
				}
				return
			}
			if strings.TrimSpace(content) == "" {
				log.Warn("agent %s skipped empty content after retries", cfg.ID)
				return
			}

			mu.Lock()
			responses = append(responses, ChatResponse{
				AgentID:   cfg.ID,
				AgentName: cfg.Name,
				Role:      cfg.Role,
				Content:   content,
			})
			mu.Unlock()
			log.Debug("agent %s done, content len: %d", cfg.ID, len(content))
		}(agentConfig)
	}

	wg.Wait()
	log.Info("all agents done, got %d responses", len(responses))
	return responses, nil
}

// runSingleAgentWithContext 运行单个 Agent（支持引用上下文）
func (s *Service) runSingleAgentWithContext(ctx context.Context, attemptTimeout time.Duration, builder *adk.ExpertAgentBuilder, cfg *models.AgentConfig, stock *models.Stock, query string, replyContent string, coreContext string, intentContext string, position *models.StockPosition) (string, error) {
	if attemptTimeout <= 0 {
		attemptTimeout = AgentTimeout
	}
	var lastErr error
	for attempt := 1; attempt <= s.retryCount; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		log.Debug(
			"agent %s attempt %d/%d start (context), deadlineLeft=%s queryLen=%d replyLen=%d coreLen=%d intentLen=%d",
			cfg.ID,
			attempt,
			s.retryCount,
			contextDeadlineLeft(attemptCtx),
			len(query),
			len(replyContent),
			len(coreContext),
			len(intentContext),
		)
		attemptStart := time.Now()
		content, err := s.runSingleAgentWithContextOnce(attemptCtx, builder, cfg, stock, query, replyContent, coreContext, intentContext, position)
		cancel()
		if err == nil {
			if strings.TrimSpace(content) == "" {
				err = fmt.Errorf("%w: agent=%s", ErrEmptyAgentReply, cfg.ID)
				log.Warn("agent %s attempt %d/%d produced empty content, elapsed=%s", cfg.ID, attempt, s.retryCount, time.Since(attemptStart).Round(time.Millisecond))
			} else {
				log.Info("agent %s attempt %d/%d succeeded, elapsed=%s contentLen=%d", cfg.ID, attempt, s.retryCount, time.Since(attemptStart).Round(time.Millisecond), len(content))
				return content, nil
			}
		}
		lastErr = err
		log.Warn(
			"agent %s attempt %d/%d failed, elapsed=%s err=%s",
			cfg.ID,
			attempt,
			s.retryCount,
			time.Since(attemptStart).Round(time.Millisecond),
			normalizeLogText(err.Error(), 320),
		)
		if !s.shouldRetry(err, ctx) || attempt == s.retryCount {
			return "", err
		}
		if errors.Is(err, ErrEmptyAgentReply) {
			log.Warn("agent %s empty content, retrying %d/%d", cfg.ID, attempt, s.retryCount)
		} else {
			log.Warn("agent %s error, retrying %d/%d: %v", cfg.ID, attempt, s.retryCount, err)
		}
		if !s.waitBeforeRetry(ctx, attempt) {
			return "", err
		}
	}
	return "", lastErr
}

func (s *Service) runSingleAgentWithContextOnce(ctx context.Context, builder *adk.ExpertAgentBuilder, cfg *models.AgentConfig, stock *models.Stock, query string, replyContent string, coreContext string, intentContext string, position *models.StockPosition) (string, error) {
	runStart := time.Now()
	agentInstance, err := builder.BuildAgentWithContext(cfg, stock, query, replyContent, coreContext, intentContext, position)
	if err != nil {
		return "", err
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "jcp",
		Agent:          agentInstance,
		SessionService: sessionService,
	})
	if err != nil {
		return "", err
	}

	sessionID := fmt.Sprintf("session-%s-%d", cfg.ID, time.Now().UnixNano())
	_, err = sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "jcp",
		UserID:    "user",
		SessionID: sessionID,
	})
	if err != nil {
		return "", fmt.Errorf("create session error: %w", err)
	}

	userInput := buildAgentUserQuery(stock, query, position)
	userMsg := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			genai.NewPartFromText(userInput),
		},
	}
	if s.verboseAgentIO {
		instruction := builder.BuildInstructionPreview(cfg, stock, query, replyContent, coreContext, intentContext, position)
		s.logAgentRequestPayload(cfg, sessionID, instruction, userInput)
	}

	var content string
	runCfg := agent.RunConfig{}
	metrics := agentRunMetrics{}
	for event, err := range r.Run(ctx, "user", sessionID, userMsg, runCfg) {
		metrics.eventCount++
		if err != nil {
			log.Error(
				"agent %s run error (context) session=%s elapsed=%s %s err=%s",
				cfg.ID,
				sessionID,
				time.Since(runStart).Round(time.Millisecond),
				metrics.summary(),
				normalizeLogText(err.Error(), 320),
			)
			return "", err
		}
		if event == nil {
			metrics.nilEventCount++
			continue
		}
		metrics.llmRespCount++
		if event.LLMResponse.Partial {
			metrics.partialCount++
		}
		if event.LLMResponse.TurnComplete {
			metrics.turnComplete++
		}
		if event.LLMResponse.FinishReason != genai.FinishReasonUnspecified {
			metrics.finishReason = fmt.Sprintf("%v", event.LLMResponse.FinishReason)
		}

		if event.LLMResponse.Content == nil {
			continue
		}
		for _, part := range event.LLMResponse.Content.Parts {
			if part.Thought {
				metrics.thoughtPartCount++
				continue
			}
			if part.FunctionCall != nil {
				metrics.toolCallCount++
				s.logAgentToolCall(cfg, sessionID, stock, part.FunctionCall)
			}
			if part.FunctionResponse != nil {
				metrics.toolResultCount++
				s.logAgentToolResult(cfg, sessionID, part.FunctionResponse)
			}
			if part.Text != "" {
				metrics.textPartCount++
				metrics.textBytes += len(part.Text)
				merged, _, changed := mergeStreamText(content, part.Text)
				if changed {
					content = merged
				}
			}
		}
	}
	content = sanitizeAgentOutput(content)
	if err := validateAgentOutputConsistency(content); err != nil {
		log.Warn(
			"agent %s run finished with inconsistent content (context) session=%s elapsed=%s err=%s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			normalizeLogText(err.Error(), 240),
		)
		return "", err
	}

	if strings.TrimSpace(content) == "" {
		log.Warn(
			"agent %s run finished with empty content (context) session=%s elapsed=%s %s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			metrics.summary(),
		)
	} else {
		log.Info(
			"agent %s run finished (context) session=%s elapsed=%s contentLen=%d %s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			len(content),
			metrics.summary(),
		)
	}
	s.logAgentFinalResponse(cfg, sessionID, content)
	return content, nil
}

// filterAgentsOrdered 按指定顺序筛选专家（保持小韭菜选择的顺序）
func (s *Service) filterAgentsOrdered(all []models.AgentConfig, ids []string) []models.AgentConfig {
	agentMap := make(map[string]models.AgentConfig)
	for _, a := range all {
		agentMap[a.ID] = a
	}
	var result []models.AgentConfig
	for _, id := range ids {
		if agent, ok := agentMap[id]; ok {
			result = append(result, agent)
		}
	}
	return result
}

func (s *Service) enforceSelectionDiversity(all []models.AgentConfig, selected []models.AgentConfig, query string, symbol string) []models.AgentConfig {
	enabled := enabledAgents(all)
	if len(enabled) == 0 {
		return selected
	}

	preferred := uniqueAgentsByID(selected, enabled)
	style := effectiveSelectionStyle(s.selectionStyle)
	target := decideTargetCount(len(enabled), len(preferred), query, style)
	if target <= 0 {
		return preferred
	}

	seed := stableSelectionSeed(query, symbol)
	pool := buildSelectionPool(enabled, preferred, seed)
	result := make([]models.AgentConfig, 0, target)
	requirements := decideSelectionRequirements(query, target)

	addOne := func(match func(models.AgentConfig) bool, allow func([]models.AgentConfig, models.AgentConfig) bool) bool {
		for _, candidate := range pool {
			if hasAgentID(result, candidate.ID) {
				continue
			}
			if !match(candidate) {
				continue
			}
			if allow != nil && !allow(result, candidate) {
				continue
			}
			result = append(result, candidate)
			return true
		}
		return false
	}

	maxAggressive := maxAggressiveByStyle(style, target)
	allowByAggressiveLimit := func(current []models.AgentConfig, candidate models.AgentConfig) bool {
		if agentStyle(candidate) != "aggressive" {
			return true
		}
		return countByStyle(current, "aggressive") < maxAggressive
	}

	// 第一层：确保核心视角覆盖
	if requirements.needRisk {
		_ = addOne(func(a models.AgentConfig) bool {
			return agentBucket(a) == "risk"
		}, allowByAggressiveLimit)
	}
	if requirements.needTechCapital {
		_ = addOne(func(a models.AgentConfig) bool {
			bucket := agentBucket(a)
			return bucket == "technical" || bucket == "capital"
		}, allowByAggressiveLimit)
	}
	if requirements.needSupplement {
		_ = addOne(func(a models.AgentConfig) bool {
			bucket := agentBucket(a)
			return bucket == "fundamental" || bucket == "valuation" || bucket == "policy" || bucket == "sentiment" || bucket == "move"
		}, allowByAggressiveLimit)
	}

	// 第二层：按“1-2 激进 + 多个稳健”补齐
	minAggressive, minStable := styleTargets(pool, target, style)
	for len(result) < target && countByStyle(result, "aggressive") < minAggressive {
		if !addOne(func(a models.AgentConfig) bool {
			return agentStyle(a) == "aggressive"
		}, allowByAggressiveLimit) {
			break
		}
	}
	for len(result) < target && countByStyle(result, "stable") < minStable {
		if !addOne(func(a models.AgentConfig) bool {
			return agentStyle(a) == "stable"
		}, allowByAggressiveLimit) {
			break
		}
	}

	// 第三层：填满目标数量
	for len(result) < target {
		stableLacking := countByStyle(result, "stable") < minStable
		aggressiveFull := countByStyle(result, "aggressive") >= maxAggressive

		var added bool
		switch {
		case stableLacking:
			added = addOne(func(a models.AgentConfig) bool {
				return agentStyle(a) == "stable"
			}, allowByAggressiveLimit)
			if !added {
				added = addOne(func(a models.AgentConfig) bool {
					return true
				}, allowByAggressiveLimit)
			}
		case aggressiveFull:
			added = addOne(func(a models.AgentConfig) bool {
				return agentStyle(a) != "aggressive"
			}, allowByAggressiveLimit)
			if !added {
				added = addOne(func(a models.AgentConfig) bool {
					return true
				}, allowByAggressiveLimit)
			}
		default:
			added = addOne(func(a models.AgentConfig) bool {
				return true
			}, allowByAggressiveLimit)
		}
		if !added {
			break
		}
	}

	if len(result) == 0 {
		return s.fallbackAgents(enabled, target)
	}
	return result
}

func enabledAgents(all []models.AgentConfig) []models.AgentConfig {
	result := make([]models.AgentConfig, 0, len(all))
	for _, agent := range all {
		if agent.Enabled {
			result = append(result, agent)
		}
	}
	return result
}

func uniqueAgentsByID(selected []models.AgentConfig, allowed []models.AgentConfig) []models.AgentConfig {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, agent := range allowed {
		allowedSet[agent.ID] = struct{}{}
	}

	seen := make(map[string]struct{}, len(selected))
	result := make([]models.AgentConfig, 0, len(selected))
	for _, agent := range selected {
		if _, ok := allowedSet[agent.ID]; !ok {
			continue
		}
		if _, ok := seen[agent.ID]; ok {
			continue
		}
		seen[agent.ID] = struct{}{}
		result = append(result, agent)
	}
	return result
}

func buildSelectionPool(enabled []models.AgentConfig, preferred []models.AgentConfig, seed uint32) []models.AgentConfig {
	priority := uniqueAgentsByID(preferred, enabled)
	remaining := rotatedRemainingCandidates(enabled, priority, seed+97)
	pool := make([]models.AgentConfig, 0, len(priority)+len(remaining))
	pool = append(pool, priority...)
	pool = append(pool, remaining...)
	return pool
}

func hasAgentID(agents []models.AgentConfig, id string) bool {
	for _, agent := range agents {
		if agent.ID == id {
			return true
		}
	}
	return false
}

func countByStyle(agents []models.AgentConfig, style string) int {
	count := 0
	for _, agent := range agents {
		if agentStyle(agent) == style {
			count++
		}
	}
	return count
}

type selectionRequirements struct {
	needRisk        bool
	needTechCapital bool
	needSupplement  bool
}

func effectiveSelectionStyle(style models.AgentSelectionStyle) models.AgentSelectionStyle {
	switch style {
	case models.AgentSelectionBalanced, models.AgentSelectionConservative, models.AgentSelectionAggressive:
		return style
	default:
		return models.AgentSelectionBalanced
	}
}

func maxAggressiveByStyle(style models.AgentSelectionStyle, target int) int {
	style = effectiveSelectionStyle(style)
	switch style {
	case models.AgentSelectionConservative:
		return 1
	case models.AgentSelectionAggressive:
		if target >= 4 {
			return 2
		}
		if target >= 3 {
			return 2
		}
		return 1
	default:
		if target <= 2 {
			return 1
		}
		return 2
	}
}

func decideTargetCount(enabledCount int, preferredCount int, query string, style models.AgentSelectionStyle) int {
	if enabledCount <= 0 {
		return 0
	}
	minTarget := 2
	if enabledCount < minTarget {
		minTarget = enabledCount
	}

	target := preferredCount
	if target < minTarget {
		target = minTarget
	}
	if target > 4 {
		target = 4
	}

	score := queryComplexityScore(query)
	switch {
	case score >= 3 && enabledCount >= 4 && target < 4:
		target = 4
	case score >= 1 && enabledCount >= 3 && target < 3:
		target = 3
	}
	style = effectiveSelectionStyle(style)
	switch style {
	case models.AgentSelectionConservative:
		if target > 3 {
			target = 3
		}
		if score == 0 && target > 2 {
			target = 2
		}
	case models.AgentSelectionAggressive:
		if enabledCount >= 3 && target < 3 {
			target = 3
		}
		if score >= 2 && enabledCount >= 4 && target < 4 {
			target = 4
		}
	}

	if target > enabledCount {
		target = enabledCount
	}
	if target < 1 {
		target = 1
	}
	return target
}

func queryComplexityScore(query string) int {
	text := strings.ToLower(strings.TrimSpace(query))
	score := 0
	if len([]rune(text)) >= 20 {
		score++
	}
	if hasAnyKeyword(text, "买", "卖", "仓位", "止损", "止盈", "操作", "建议", "空间", "风险") {
		score++
	}
	if hasAnyKeyword(text, "今天", "明天", "短线", "盘中", "回踩", "突破", "追高", "技术", "资金") {
		score++
	}
	if hasAnyKeyword(text, "估值", "基本面", "财报", "政策", "题材", "行业", "长期") {
		score++
	}
	return score
}

func decideSelectionRequirements(query string, target int) selectionRequirements {
	text := strings.ToLower(strings.TrimSpace(query))
	decisionLike := hasAnyKeyword(text, "买", "卖", "仓位", "止损", "止盈", "加仓", "减仓", "建议", "操作", "空间")
	shortTermLike := hasAnyKeyword(text, "今天", "明天", "短线", "盘中", "追高", "回踩", "突破", "k线", "技术", "资金")
	longTermLike := hasAnyKeyword(text, "中线", "长线", "长期", "基本面", "估值", "财报", "政策", "题材", "行业")

	return selectionRequirements{
		needRisk:        decisionLike,
		needTechCapital: shortTermLike || decisionLike,
		needSupplement:  target >= 3 || longTermLike,
	}
}

func styleTargets(pool []models.AgentConfig, target int, style models.AgentSelectionStyle) (minAggressive int, minStable int) {
	if target <= 0 {
		return 0, 0
	}

	hasAggressive := false
	hasStable := false
	aggressiveCount := 0
	stableCount := 0
	for _, agent := range pool {
		switch agentStyle(agent) {
		case "aggressive":
			hasAggressive = true
			aggressiveCount++
		case "stable":
			hasStable = true
			stableCount++
		}
	}

	style = effectiveSelectionStyle(style)

	if hasAggressive && target >= 2 {
		minAggressive = 1
		if style == models.AgentSelectionAggressive && target >= 4 {
			minAggressive = 2
		}
		if style == models.AgentSelectionConservative {
			minAggressive = 1
			if target <= 2 {
				minAggressive = 0
			}
		}
	}

	if hasStable {
		minStable = 1
		if target >= 3 {
			minStable = 2
		}
		if style == models.AgentSelectionConservative {
			minStable = target - minAggressive
		}
		if style == models.AgentSelectionAggressive {
			minStable = 1
		}
		if target == 1 {
			minStable = 1
		}
		limit := target - minAggressive
		if limit < 0 {
			limit = 0
		}
		if minStable > limit {
			minStable = limit
		}
		if minStable == 0 && target > 0 {
			minStable = 1
		}
		if minStable > stableCount {
			minStable = stableCount
		}
	}
	if minAggressive > aggressiveCount {
		minAggressive = aggressiveCount
	}

	return minAggressive, minStable
}

func containsBucket(agents []models.AgentConfig, bucket string) bool {
	for _, agent := range agents {
		if agentBucket(agent) == bucket {
			return true
		}
	}
	return false
}

func containsAnyBucket(agents []models.AgentConfig, buckets ...string) bool {
	for _, bucket := range buckets {
		if containsBucket(agents, bucket) {
			return true
		}
	}
	return false
}

func pickByBuckets(all []models.AgentConfig, selected []models.AgentConfig, seed uint32, buckets ...string) (models.AgentConfig, bool) {
	want := make(map[string]struct{}, len(buckets))
	for _, bucket := range buckets {
		want[bucket] = struct{}{}
	}

	for _, candidate := range rotatedRemainingCandidates(all, selected, seed) {
		if _, ok := want[agentBucket(candidate)]; ok {
			return candidate, true
		}
	}
	return models.AgentConfig{}, false
}

func pickMostDiverseAgent(all []models.AgentConfig, selected []models.AgentConfig, seed uint32) (models.AgentConfig, bool) {
	candidates := rotatedRemainingCandidates(all, selected, seed)
	if len(candidates) == 0 {
		return models.AgentConfig{}, false
	}

	bucketCount := make(map[string]int)
	for _, agent := range selected {
		bucketCount[agentBucket(agent)]++
	}

	best := candidates[0]
	bestBucket := agentBucket(best)
	bestCount := bucketCount[bestBucket]
	bestPriority := bucketPriority(bestBucket)

	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		candidateBucket := agentBucket(candidate)
		candidateCount := bucketCount[candidateBucket]
		candidatePriority := bucketPriority(candidateBucket)

		if candidateCount < bestCount {
			best = candidate
			bestBucket = candidateBucket
			bestCount = candidateCount
			bestPriority = candidatePriority
			continue
		}
		if candidateCount == bestCount && candidatePriority < bestPriority {
			best = candidate
			bestBucket = candidateBucket
			bestCount = candidateCount
			bestPriority = candidatePriority
		}
	}

	return best, true
}

func rotatedRemainingCandidates(all []models.AgentConfig, selected []models.AgentConfig, seed uint32) []models.AgentConfig {
	selectedSet := make(map[string]struct{}, len(selected))
	for _, agent := range selected {
		selectedSet[agent.ID] = struct{}{}
	}

	candidates := make([]models.AgentConfig, 0, len(all))
	for _, agent := range all {
		if _, exists := selectedSet[agent.ID]; exists {
			continue
		}
		candidates = append(candidates, agent)
	}
	if len(candidates) <= 1 {
		return candidates
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ID == candidates[j].ID {
			return candidates[i].Name < candidates[j].Name
		}
		return candidates[i].ID < candidates[j].ID
	})

	offset := int(seed % uint32(len(candidates)))
	if offset == 0 {
		return candidates
	}
	rotated := make([]models.AgentConfig, 0, len(candidates))
	rotated = append(rotated, candidates[offset:]...)
	rotated = append(rotated, candidates[:offset]...)
	return rotated
}

func bucketPriority(bucket string) int {
	switch bucket {
	case "fundamental", "valuation", "policy", "sentiment", "move":
		return 0
	case "other":
		return 1
	default:
		return 2
	}
}

func agentBucket(agent models.AgentConfig) string {
	combined := strings.ToLower(strings.Join(append([]string{
		agent.ID,
		agent.Name,
		agent.Role,
	}, agent.Tools...), " "))

	switch {
	case hasAnyKeyword(combined, "风控", "风险", "risk", "pledge", "lockup", "buyback"):
		return "risk"
	case hasAnyKeyword(combined, "k线", "技术", "technical", "macd", "rsi", "get_kline_data"):
		return "technical"
	case hasAnyKeyword(combined, "资金", "龙虎", "capital", "fund_flow", "longhubang"):
		return "capital"
	case hasAnyKeyword(combined, "基本面", "财务", "fundamental", "financial", "business"):
		return "fundamental"
	case hasAnyKeyword(combined, "估值", "valuation", "pe", "pb"):
		return "valuation"
	case hasAnyKeyword(combined, "政策", "题材", "policy", "core_themes"):
		return "policy"
	case hasAnyKeyword(combined, "舆情", "情绪", "hottrend", "orderbook"):
		return "sentiment"
	case hasAnyKeyword(combined, "异动", "move", "board_leaders", "stock_moves"):
		return "move"
	default:
		return "other"
	}
}

func agentStyle(agent models.AgentConfig) string {
	switch agentBucket(agent) {
	case "technical", "capital", "move", "sentiment":
		return "aggressive"
	case "risk", "fundamental", "valuation", "policy":
		return "stable"
	default:
		return "neutral"
	}
}

func hasAnyKeyword(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func stableSelectionSeed(query string, symbol string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.ToLower(strings.TrimSpace(symbol))))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(strings.ToLower(strings.TrimSpace(query))))
	return hasher.Sum32()
}

func extractAgentIDs(agents []models.AgentConfig) []string {
	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		ids = append(ids, agent.ID)
	}
	return ids
}

// fallbackAgents 当主持人选人失败时兜底选择前N位已启用专家
func (s *Service) fallbackAgents(all []models.AgentConfig, limit int) []models.AgentConfig {
	if limit <= 0 {
		limit = 1
	}

	result := make([]models.AgentConfig, 0, limit)
	for _, agent := range all {
		if !agent.Enabled {
			continue
		}
		result = append(result, agent)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// buildPreviousContext 构建前面专家发言的上下文
func (s *Service) buildPreviousContext(history []DiscussionEntry) string {
	if len(history) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("【前面专家的发言】\n")
	for _, entry := range history {
		sb.WriteString(fmt.Sprintf("- %s（%s）：%s\n\n", entry.AgentName, entry.Role, entry.Content))
	}
	return sb.String()
}

// extractKeyPointsFromHistory 从讨论历史中提取关键点
func (s *Service) extractKeyPointsFromHistory(ctx context.Context, history []DiscussionEntry) []string {
	// 如果有记忆管理器，使用 LLM 智能提取
	if s.memoryManager != nil {
		discussions := make([]memory.DiscussionInput, 0, len(history))
		for _, entry := range history {
			discussions = append(discussions, memory.DiscussionInput{
				AgentName: entry.AgentName,
				Role:      entry.Role,
				Content:   entry.Content,
			})
		}
		keyPoints, err := s.memoryManager.ExtractKeyPoints(ctx, discussions)
		if err != nil {
			log.Warn("LLM extract key points error, fallback: %v", err)
		} else {
			return keyPoints
		}
	}

	// 降级：简单截取
	keyPoints := make([]string, 0, len(history))
	for _, entry := range history {
		runes := []rune(entry.Content)
		content := entry.Content
		if len(runes) > 80 {
			content = string(runes[:80]) + "..."
		}
		keyPoints = append(keyPoints, fmt.Sprintf("%s: %s", entry.AgentName, content))
	}
	return keyPoints
}

// runSingleAgentWithHistory 运行单个专家（带历史上下文和进度回调）
func (s *Service) runSingleAgentWithHistory(
	ctx context.Context,
	attemptTimeout time.Duration,
	builder *adk.ExpertAgentBuilder,
	cfg *models.AgentConfig,
	stock *models.Stock,
	query string,
	previousContext string,
	coreContext string,
	intentContext string,
	progressCallback ProgressCallback,
	position *models.StockPosition,
) (string, error) {
	if attemptTimeout <= 0 {
		attemptTimeout = AgentTimeout
	}
	var lastErr error
	for attempt := 1; attempt <= s.retryCount; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		log.Debug(
			"agent %s attempt %d/%d start (history), deadlineLeft=%s queryLen=%d prevLen=%d coreLen=%d intentLen=%d",
			cfg.ID,
			attempt,
			s.retryCount,
			contextDeadlineLeft(attemptCtx),
			len(query),
			len(previousContext),
			len(coreContext),
			len(intentContext),
		)
		attemptStart := time.Now()
		content, err := s.runSingleAgentWithHistoryOnce(attemptCtx, builder, cfg, stock, query, previousContext, coreContext, intentContext, progressCallback, position)
		cancel()
		if err == nil {
			if strings.TrimSpace(content) == "" {
				err = fmt.Errorf("%w: agent=%s", ErrEmptyAgentReply, cfg.ID)
				log.Warn("agent %s attempt %d/%d produced empty content, elapsed=%s", cfg.ID, attempt, s.retryCount, time.Since(attemptStart).Round(time.Millisecond))
			} else {
				log.Info("agent %s attempt %d/%d succeeded, elapsed=%s contentLen=%d", cfg.ID, attempt, s.retryCount, time.Since(attemptStart).Round(time.Millisecond), len(content))
				return content, nil
			}
		}
		lastErr = err
		log.Warn(
			"agent %s attempt %d/%d failed, elapsed=%s err=%s",
			cfg.ID,
			attempt,
			s.retryCount,
			time.Since(attemptStart).Round(time.Millisecond),
			normalizeLogText(err.Error(), 320),
		)
		if !s.shouldRetry(err, ctx) || attempt == s.retryCount {
			return "", err
		}
		if errors.Is(err, ErrEmptyAgentReply) {
			log.Warn("agent %s empty content, retrying %d/%d", cfg.ID, attempt, s.retryCount)
		} else {
			log.Warn("agent %s error, retrying %d/%d: %v", cfg.ID, attempt, s.retryCount, err)
		}
		if !s.waitBeforeRetry(ctx, attempt) {
			return "", err
		}
	}
	return "", lastErr
}

func (s *Service) runSingleAgentWithHistoryOnce(
	ctx context.Context,
	builder *adk.ExpertAgentBuilder,
	cfg *models.AgentConfig,
	stock *models.Stock,
	query string,
	previousContext string,
	coreContext string,
	intentContext string,
	progressCallback ProgressCallback,
	position *models.StockPosition,
) (string, error) {
	runStart := time.Now()
	// 使用带上下文的方法构建 Agent
	agentInstance, err := builder.BuildAgentWithContext(cfg, stock, query, previousContext, coreContext, intentContext, position)
	if err != nil {
		return "", err
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "jcp",
		Agent:          agentInstance,
		SessionService: sessionService,
	})
	if err != nil {
		return "", err
	}

	sessionID := fmt.Sprintf("session-%s-%d", cfg.ID, time.Now().UnixNano())
	_, err = sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "jcp",
		UserID:    "user",
		SessionID: sessionID,
	})
	if err != nil {
		return "", fmt.Errorf("create session error: %w", err)
	}

	userInput := buildAgentUserQuery(stock, query, position)
	userMsg := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			genai.NewPartFromText(userInput),
		},
	}
	if s.verboseAgentIO {
		instruction := builder.BuildInstructionPreview(cfg, stock, query, previousContext, coreContext, intentContext, position)
		s.logAgentRequestPayload(cfg, sessionID, instruction, userInput)
	}

	var content string
	receivedPartial := false
	metrics := agentRunMetrics{}
	runCfg := agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}
	for event, err := range r.Run(ctx, "user", sessionID, userMsg, runCfg) {
		metrics.eventCount++
		if err != nil {
			log.Error(
				"agent %s run error (history) session=%s elapsed=%s %s err=%s",
				cfg.ID,
				sessionID,
				time.Since(runStart).Round(time.Millisecond),
				metrics.summary(),
				normalizeLogText(err.Error(), 320),
			)
			return "", err
		}
		if event == nil {
			metrics.nilEventCount++
			continue
		}
		metrics.llmRespCount++
		if event.LLMResponse.Partial {
			metrics.partialCount++
		}
		if event.LLMResponse.TurnComplete {
			metrics.turnComplete++
		}
		if event.LLMResponse.FinishReason != genai.FinishReasonUnspecified {
			metrics.finishReason = fmt.Sprintf("%v", event.LLMResponse.FinishReason)
		}
		if event.LLMResponse.Content == nil {
			continue
		}

		for _, part := range event.LLMResponse.Content.Parts {
			if part.Thought {
				metrics.thoughtPartCount++
				continue
			}
			if part.FunctionCall != nil {
				s.logAgentToolCall(cfg, sessionID, stock, part.FunctionCall)
			}
			if part.FunctionResponse != nil {
				s.logAgentToolResult(cfg, sessionID, part.FunctionResponse)
			}

			// 检测工具调用
			if part.FunctionCall != nil && progressCallback != nil {
				metrics.toolCallCount++
				log.Debug("agent %s tool_call session=%s tool=%s", cfg.ID, sessionID, part.FunctionCall.Name)
				progressCallback(ProgressEvent{
					Type:      "tool_call",
					AgentID:   cfg.ID,
					AgentName: cfg.Name,
					Detail:    part.FunctionCall.Name,
				})
			}
			if part.FunctionCall != nil && progressCallback == nil {
				metrics.toolCallCount++
			}

			// 检测工具结果
			if part.FunctionResponse != nil && progressCallback != nil {
				metrics.toolResultCount++
				log.Debug("agent %s tool_result session=%s tool=%s", cfg.ID, sessionID, part.FunctionResponse.Name)
				progressCallback(ProgressEvent{
					Type:      "tool_result",
					AgentID:   cfg.ID,
					AgentName: cfg.Name,
					Detail:    part.FunctionResponse.Name,
				})
			}
			if part.FunctionResponse != nil && progressCallback == nil {
				metrics.toolResultCount++
			}

			// 流式文本：只累积 Partial 片段，忽略最终聚合响应（避免重复）
			if part.Text != "" {
				metrics.textPartCount++
				metrics.textBytes += len(part.Text)
				if event.LLMResponse.Partial {
					receivedPartial = true
					merged, delta, changed := mergeStreamText(content, part.Text)
					if changed {
						content = merged
					}
					if progressCallback != nil && delta != "" {
						progressCallback(ProgressEvent{
							Type:      "streaming",
							AgentID:   cfg.ID,
							AgentName: cfg.Name,
							Content:   delta,
						})
					}
				} else if !receivedPartial {
					merged, _, changed := mergeStreamText(content, part.Text)
					if changed {
						content = merged
					}
				}
			}
		}
	}
	content = sanitizeAgentOutput(content)
	if err := validateAgentOutputConsistency(content); err != nil {
		log.Warn(
			"agent %s run finished with inconsistent content (history) session=%s elapsed=%s err=%s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			normalizeLogText(err.Error(), 240),
		)
		return "", err
	}

	if strings.TrimSpace(content) == "" {
		log.Warn(
			"agent %s run finished with empty content (history) session=%s elapsed=%s partialReceived=%t %s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			receivedPartial,
			metrics.summary(),
		)
	} else {
		log.Info(
			"agent %s run finished (history) session=%s elapsed=%s contentLen=%d partialReceived=%t %s",
			cfg.ID,
			sessionID,
			time.Since(runStart).Round(time.Millisecond),
			len(content),
			receivedPartial,
			metrics.summary(),
		)
	}
	s.logAgentFinalResponse(cfg, sessionID, content)
	return content, nil
}

func (s *Service) buildSummaryContext(coreContext, intentContext, supplementContext string, stock *models.Stock, position *models.StockPosition) string {
	var sb strings.Builder
	if position != nil && position.Shares > 0 {
		currentPrice := 0.0
		symbol := ""
		name := ""
		if stock != nil {
			currentPrice = stock.Price
			symbol = strings.TrimSpace(stock.Symbol)
			name = strings.TrimSpace(stock.Name)
		}
		stockLabel := strings.TrimSpace(strings.TrimSpace(name + " " + symbol))
		if stockLabel == "" {
			stockLabel = "当前标的"
		}
		pnlText := "暂无盈亏数据"
		if currentPrice > 0 {
			pnlPerShare := currentPrice - position.CostPrice
			pnlTotal := pnlPerShare * float64(position.Shares)
			pnlRatio := 0.0
			if position.CostPrice > 0 {
				pnlRatio = pnlPerShare / position.CostPrice * 100
			}
			pnlText = fmt.Sprintf("浮盈亏 %.2f（%.2f%%）", pnlTotal, pnlRatio)
		}
		sb.WriteString("【用户持仓】\n")
		sb.WriteString(fmt.Sprintf("%s：持有 %d 股，成本价 %.2f，现价 %.2f，%s。\n", stockLabel, position.Shares, position.CostPrice, currentPrice, pnlText))
	}
	if coreContext != "" {
		sb.WriteString("【核心数据包】\n")
		sb.WriteString(coreContext)
		sb.WriteString("\n")
	}
	if intentContext != "" {
		sb.WriteString("【意图补充数据】\n")
		sb.WriteString(intentContext)
		sb.WriteString("\n")
	}
	if supplementContext != "" {
		sb.WriteString("【二次补充数据】\n")
		sb.WriteString(supplementContext)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func (s *Service) responsesToHistory(responses []ChatResponse) []DiscussionEntry {
	history := make([]DiscussionEntry, 0, len(responses))
	for _, resp := range responses {
		history = append(history, DiscussionEntry{
			Round:     resp.Round,
			AgentID:   resp.AgentID,
			AgentName: resp.AgentName,
			Role:      resp.Role,
			Content:   resp.Content,
		})
	}
	return history
}

// SummarizeDirect 为直连模式生成小韭菜总结
func (s *Service) SummarizeDirect(ctx context.Context, aiConfig *models.AIConfig, stock *models.Stock, query string, responses []ChatResponse, coreContext string, intentContext string, position *models.StockPosition) (string, error) {
	if aiConfig == nil {
		return "", ErrNoAIConfig
	}
	if len(responses) == 0 {
		return "", nil
	}

	llm, err := s.modelFactory.CreateModel(ctx, aiConfig)
	if err != nil {
		return "", err
	}

	// 创建 Moderator LLM（优先使用独立配置）
	var moderatorLLM model.LLM
	if s.moderatorAIConfig != nil {
		moderatorLLM, err = s.modelFactory.CreateModel(ctx, s.moderatorAIConfig)
		if err != nil {
			log.Warn("create moderator LLM error, fallback to default: %v", err)
			moderatorLLM = llm
		} else {
			log.Debug("using dedicated moderator LLM: %s", s.moderatorAIConfig.ModelName)
		}
	} else {
		moderatorLLM = llm
	}

	moderator := NewModerator(moderatorLLM)
	history := s.responsesToHistory(responses)
	extraContext := s.buildSummaryContext(coreContext, intentContext, "", stock, position)
	return s.summarizeWithRetry(ctx, moderator, stock, query, history, extraContext)
}

// createBuilder 创建 ExpertAgentBuilder
func (s *Service) createBuilder(llm model.LLM, aiConfig *models.AIConfig) *adk.ExpertAgentBuilder {
	if s.mcpManager != nil {
		return adk.NewExpertAgentBuilderFull(llm, aiConfig, s.toolRegistry, s.mcpManager)
	}
	if s.toolRegistry != nil {
		return adk.NewExpertAgentBuilderWithTools(llm, aiConfig, s.toolRegistry)
	}
	return adk.NewExpertAgentBuilder(llm, aiConfig)
}
