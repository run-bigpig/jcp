package adk

import (
	"fmt"
	"strings"
	"time"

	"github.com/run-bigpig/jcp/internal/adk/mcp"
	"github.com/run-bigpig/jcp/internal/adk/tools"
	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

// ExpertAgentBuilder 专家 Agent 构建器
type ExpertAgentBuilder struct {
	llm          model.LLM
	aiConfig     *models.AIConfig // AI 配置（包含 temperature、maxTokens）
	toolRegistry *tools.Registry
	mcpManager   *mcp.Manager
}

// NewExpertAgentBuilder 创建专家 Agent 构建器
func NewExpertAgentBuilder(llm model.LLM, aiConfig *models.AIConfig) *ExpertAgentBuilder {
	return &ExpertAgentBuilder{llm: llm, aiConfig: aiConfig}
}

// NewExpertAgentBuilderWithTools 创建带工具的专家 Agent 构建器
func NewExpertAgentBuilderWithTools(llm model.LLM, aiConfig *models.AIConfig, registry *tools.Registry) *ExpertAgentBuilder {
	return &ExpertAgentBuilder{llm: llm, aiConfig: aiConfig, toolRegistry: registry}
}

// NewExpertAgentBuilderFull 创建完整配置的专家 Agent 构建器
func NewExpertAgentBuilderFull(llm model.LLM, aiConfig *models.AIConfig, registry *tools.Registry, mcpMgr *mcp.Manager) *ExpertAgentBuilder {
	return &ExpertAgentBuilder{llm: llm, aiConfig: aiConfig, toolRegistry: registry, mcpManager: mcpMgr}
}

// BuildAgent 根据配置构建 LLM Agent
func (b *ExpertAgentBuilder) BuildAgent(config *models.AgentConfig, stock *models.Stock, query string, position *models.StockPosition) (agent.Agent, error) {
	return b.BuildAgentWithContext(config, stock, query, "", "", "", position)
}

// BuildAgentWithContext 根据配置构建 LLM Agent（支持引用上下文）
func (b *ExpertAgentBuilder) BuildAgentWithContext(config *models.AgentConfig, stock *models.Stock, query string, replyContent string, coreContext string, intentContext string, position *models.StockPosition) (agent.Agent, error) {
	instruction := b.buildInstructionWithContext(config, stock, query, replyContent, coreContext, intentContext, position)

	// 获取 Agent 配置的工具
	var agentTools []tool.Tool
	if b.toolRegistry != nil && len(config.Tools) > 0 {
		agentTools = b.toolRegistry.GetTools(config.Tools)
	}

	// 获取 MCP toolsets
	var toolsets []tool.Toolset
	if b.mcpManager != nil && len(config.MCPServers) > 0 {
		log.Info("Agent %s 请求 MCP servers: %v", config.ID, config.MCPServers)
		toolsets = b.mcpManager.GetToolsetsByIDs(config.MCPServers)
		log.Info("Agent %s 获取到 %d 个 toolsets", config.ID, len(toolsets))
		// 打印每个 toolset 的名称
		for i, ts := range toolsets {
			log.Info("Agent %s toolset[%d]: %s", config.ID, i, ts.Name())
		}
	}

	// 构建生成配置（应用 temperature 和 maxTokens）
	var generateConfig *genai.GenerateContentConfig
	if b.aiConfig != nil {
		generateConfig = &genai.GenerateContentConfig{
			MaxOutputTokens: int32(b.aiConfig.MaxTokens),
		}
		if !shouldSkipSamplingByModel(b.aiConfig.ModelName) {
			temp := float32(b.aiConfig.Temperature)
			generateConfig.Temperature = &temp
		}
	}

	var beforeModelCallbacks []llmagent.BeforeModelCallback
	if b.aiConfig != nil && shouldSkipSamplingByModel(b.aiConfig.ModelName) {
		beforeModelCallbacks = append(beforeModelCallbacks, func(ctx agent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
			if req.Config == nil {
				req.Config = &genai.GenerateContentConfig{}
			}
			one := float32(1)
			req.Config.Temperature = &one
			req.Config.TopP = &one
			return nil, nil
		})
	}

	return llmagent.New(llmagent.Config{
		Name:                  config.ID,
		Model:                 b.llm,
		Description:           config.Role,
		Instruction:           instruction,
		Tools:                 agentTools,
		Toolsets:              toolsets,
		GenerateContentConfig: generateConfig,
		BeforeModelCallbacks:  beforeModelCallbacks,
	})
}

// BuildInstructionPreview 返回构建后的完整指令文本，便于调试日志与排障。
func (b *ExpertAgentBuilder) BuildInstructionPreview(config *models.AgentConfig, stock *models.Stock, query string, replyContent string, coreContext string, intentContext string, position *models.StockPosition) string {
	return b.buildInstructionWithContext(config, stock, query, replyContent, coreContext, intentContext, position)
}

func shouldSkipSamplingByModel(modelName string) bool {
	name := strings.ToLower(modelName)
	if strings.HasPrefix(name, "o1") || strings.HasPrefix(name, "o3") || strings.HasPrefix(name, "o4") {
		return true
	}
	if strings.HasPrefix(name, "gpt-5") || strings.HasPrefix(name, "gpt5") {
		return true
	}
	return false
}

// buildInstruction 构建 Agent 指令
func (b *ExpertAgentBuilder) buildInstruction(config *models.AgentConfig, stock *models.Stock, query string, position *models.StockPosition) string {
	return b.buildInstructionWithContext(config, stock, query, "", "", "", position)
}

// buildInstructionWithContext 构建 Agent 指令（支持引用上下文）
func (b *ExpertAgentBuilder) buildInstructionWithContext(config *models.AgentConfig, stock *models.Stock, query string, replyContent string, coreContext string, intentContext string, position *models.StockPosition) string {
	baseInstruction := config.Instruction
	if baseInstruction == "" {
		baseInstruction = fmt.Sprintf("你是一位%s，名字是%s。", config.Role, config.Name)
	}

	// 构建可用工具说明
	toolsDescription := b.buildToolsDescription(config)

	// 获取当前时间和盘中状态
	now := time.Now()
	timeStr := now.Format("2006-01-02 15:04:05")
	weekday := now.Weekday()
	hour, minute := now.Hour(), now.Minute()
	currentMinutes := hour*60 + minute

	// 判断盘中状态（A股交易时间：9:30-11:30, 13:00-15:00，周一至周五）
	var marketStatus string
	if weekday == time.Saturday || weekday == time.Sunday {
		marketStatus = "休市（周末）"
	} else if currentMinutes >= 9*60+30 && currentMinutes <= 11*60+30 {
		marketStatus = "盘中（上午交易时段）"
	} else if currentMinutes >= 13*60 && currentMinutes <= 15*60 {
		marketStatus = "盘中（下午交易时段）"
	} else if currentMinutes < 9*60+30 {
		marketStatus = "盘前"
	} else if currentMinutes > 15*60 {
		marketStatus = "盘后"
	} else {
		marketStatus = "午间休市"
	}

	prompt := fmt.Sprintf(`%s
%s
当前时间: %s
市场状态: %s

股票: %s (%s)
当前价格: %.2f
涨跌幅: %.2f%%
`, baseInstruction, toolsDescription, timeStr, marketStatus, stock.Symbol, stock.Name, stock.Price, stock.ChangePercent)

	if position != nil && position.Shares > 0 {
		costAmount := float64(position.Shares) * position.CostPrice
		marketValue := float64(position.Shares) * stock.Price
		pnlValue := marketValue - costAmount
		pnlRatio := 0.0
		if costAmount > 0 {
			pnlRatio = (pnlValue / costAmount) * 100
		}
		prompt += fmt.Sprintf(
			"\n【用户持仓】\n持有 %d 股，成本价 %.2f，按当前价 %.2f 估算浮盈亏 %.2f（%.2f%%）。\n请把补仓/减仓/止损与仓位控制作为优先分析维度。\n",
			position.Shares,
			position.CostPrice,
			stock.Price,
			pnlValue,
			pnlRatio,
		)
	}

	if strings.TrimSpace(stock.Symbol) != "" || strings.TrimSpace(stock.Name) != "" {
		prompt += fmt.Sprintf("\n【硬性约束】本轮讨论标的是 %s (%s)。除非代码和名称都为空，否则禁止要求用户再次提供股票代码/名称。\n", stock.Symbol, stock.Name)
	}
	if b.requiresMarketCodeArguments(config) {
		prompt += fmt.Sprintf(
			"\n【工具调用参数硬约束】\n"+
				"- 调用 get_kline_data 时必须带参数：{\"code\":\"%s\",\"period\":\"1d\",\"days\":60}\n"+
				"- 调用 get_stock_realtime 时必须带参数：{\"codes\":[\"%s\"]}\n"+
				"- 若分析包含“分时/盘中/承接”判断，需额外调用 get_kline_data：{\"code\":\"%s\",\"period\":\"5m\",\"days\":2}\n"+
				"- get_market_status 可空参数调用\n"+
				"- 禁止空参数调用 get_kline_data / get_stock_realtime\n",
			stock.Symbol,
			stock.Symbol,
			stock.Symbol,
		)
	}
	if b.hasTool(config, "get_orderbook") {
		prompt += fmt.Sprintf(
			"\n【盘口工具约束】\n"+
				"- 调用 get_orderbook 时必须携带当前股票代码：{\"code\":\"%s\"}\n"+
				"- 若结论涉及分时承接/买卖盘强弱，必须先调用 get_orderbook；仅当工具返回空或报错时，才可写“未获取到分时承接细节”\n"+
				"- 不要空参数调用 get_orderbook\n",
			stock.Symbol,
		)
	}
	if b.hasTool(config, "get_index_fund_flow") {
		indexCode := preferredIndexCodeByStock(stock.Symbol)
		prompt += fmt.Sprintf(
			"\n【指数资金流工具约束】\n"+
				"- 调用 get_index_fund_flow 必须带 code 参数，建议：{\"code\":\"%s\",\"interval\":\"1\",\"limit\":120}\n"+
				"- 不要空参数调用 get_index_fund_flow\n",
			indexCode,
		)
	}
	if b.hasTool(config, "get_stock_announcements") {
		prompt += fmt.Sprintf(
			"\n【公告工具约束】\n"+
				"- 调用 get_stock_announcements 时必须携带当前股票代码：{\"code\":\"%s\",\"page\":1,\"pageSize\":10}\n"+
				"- 不要空参数调用 get_stock_announcements\n",
			stock.Symbol,
		)
	}

	if coreContext != "" {
		prompt += fmt.Sprintf(`
【核心数据包】
%s
`, coreContext)
	}

	if intentContext != "" {
		prompt += fmt.Sprintf(`
【意图补充数据】
%s
`, intentContext)
	}

	// 如果有引用内容，加入上下文
	if replyContent != "" {
		prompt += fmt.Sprintf(`--- 引用的观点 ---
%s
---

小韭菜问题: %s

请结合以上引用的观点，发表你的看法。可以赞同、补充或反驳。

请按 Markdown 输出，严格使用以下结构（标题必须单独一行，顺序固定，每个标题只出现一次）：
## 结论
> 这里写结论（仅“结论”段允许使用引用格式）
## 理由
## 触发与风控
## 失效条件

格式约束（必须遵守）：
- 不要输出“参考依据”区块，不要角标 [1][2]，不要原样堆砌工具字段名。
- 不要使用代码块、行内代码、表格。
- 不要输出任何列表编号或混合编号（禁止 1) / (1) / 1. / 一、）。
- 小标题必须单独一行，不要把“标题：正文”写在同一行。
- 重点只用 **加粗**，不要使用除“结论段引用”之外的引用格式。
- 四段内容前后必须一致：若“结论”是观望/持有/减仓，则“触发与风控”不得给出无条件买入或加仓；需要反转时写成条件触发，并放入“失效条件”。

你必须提供至少1条“前面专家没有提过”的新增判断或反证；禁止复述前文原句。
若你总体结论与前文一致，必须补充新的数据依据或新的时间口径来支撑一致结论。

请补充触发条件、风险与失效条件。
总字数控制在180-360字；严禁编造数据，缺失数据请明确写“未获取到”。`, replyContent, query)
	} else {
		prompt += fmt.Sprintf(`小韭菜问题: %s

请用简洁专业的语言回答。

请按 Markdown 输出，严格使用以下结构（标题必须单独一行，顺序固定，每个标题只出现一次）：
## 结论
> 这里写结论（仅“结论”段允许使用引用格式）
## 理由
## 触发与风控
## 失效条件

格式约束（必须遵守）：
- 不要输出“参考依据”区块，不要角标 [1][2]，不要原样堆砌工具字段名。
- 不要使用代码块、行内代码、表格。
- 不要输出任何列表编号或混合编号（禁止 1) / (1) / 1. / 一、）。
- 小标题必须单独一行，不要把“标题：正文”写在同一行。
- 重点只用 **加粗**，不要使用除“结论段引用”之外的引用格式。
- 四段内容前后必须一致：若“结论”是观望/持有/减仓，则“触发与风控”不得给出无条件买入或加仓；需要反转时写成条件触发，并放入“失效条件”。

请补充触发条件、风险与失效条件。
总字数控制在180-360字；严禁编造数据，缺失数据请明确写“未获取到”。`, query)
	}

	return prompt
}

// buildToolsDescription 构建可用工具说明
func (b *ExpertAgentBuilder) buildToolsDescription(config *models.AgentConfig) string {
	var searchTools []string // 搜索类工具
	var dataTools []string   // 数据查询工具
	var otherTools []string  // 其他工具

	// 搜索类工具关键词
	searchKeywords := []string{"search", "搜索", "web", "网页", "tavily", "google", "bing"}

	// 获取内置工具信息并分类
	if b.toolRegistry != nil && len(config.Tools) > 0 {
		toolInfos := b.toolRegistry.GetToolInfosByNames(config.Tools)
		for _, info := range toolInfos {
			desc := fmt.Sprintf("- %s: %s", info.Name, info.Description)
			if b.isSearchTool(info.Name, info.Description, searchKeywords) {
				searchTools = append(searchTools, desc)
			} else if b.isDataTool(info.Name) {
				dataTools = append(dataTools, desc)
			} else {
				otherTools = append(otherTools, desc)
			}
		}
	}

	// 获取 MCP 工具信息并分类
	if b.mcpManager != nil && len(config.MCPServers) > 0 {
		mcpTools := b.mcpManager.GetToolInfosByServerIDs(config.MCPServers)
		for _, info := range mcpTools {
			desc := fmt.Sprintf("- %s: %s (来自 %s)", info.Name, info.Description, info.ServerName)
			if b.isSearchTool(info.Name, info.Description, searchKeywords) {
				searchTools = append(searchTools, desc)
			} else if b.isDataTool(info.Name) {
				dataTools = append(dataTools, desc)
			} else {
				otherTools = append(otherTools, desc)
			}
		}
	}

	if len(searchTools) == 0 && len(dataTools) == 0 && len(otherTools) == 0 {
		return ""
	}

	return b.formatToolsInstruction(searchTools, dataTools, otherTools)
}

// isSearchTool 判断是否为搜索类工具
func (b *ExpertAgentBuilder) isSearchTool(name, description string, keywords []string) bool {
	nameLower := strings.ToLower(name)
	descLower := strings.ToLower(description)
	for _, kw := range keywords {
		if strings.Contains(nameLower, kw) || strings.Contains(descLower, kw) {
			return true
		}
	}
	return false
}

// isDataTool 判断是否为数据查询工具
func (b *ExpertAgentBuilder) isDataTool(name string) bool {
	dataKeywords := []string{"kline", "k线", "realtime", "实时", "orderbook", "盘口", "news", "新闻"}
	nameLower := strings.ToLower(name)
	for _, kw := range dataKeywords {
		if strings.Contains(nameLower, kw) {
			return true
		}
	}
	return false
}

// formatToolsInstruction 格式化工具使用指导
func (b *ExpertAgentBuilder) formatToolsInstruction(searchTools, dataTools, otherTools []string) string {
	var result strings.Builder

	result.WriteString("\n## 工具使用规则（必须遵守）\n\n")

	// 搜索工具 - 强制使用
	if len(searchTools) > 0 {
		result.WriteString("### 搜索工具（遇到信息查询必须调用）\n")
		for _, t := range searchTools {
			result.WriteString(t + "\n")
		}
		result.WriteString("\n**重要**: 当用户询问新闻、事件、公告、研报、市场动态等信息时，")
		result.WriteString("你**必须先调用搜索工具**获取最新信息，**禁止凭记忆回答**。\n\n")
	}

	// 数据工具
	if len(dataTools) > 0 {
		result.WriteString("### 数据查询工具\n")
		for _, t := range dataTools {
			result.WriteString(t + "\n")
		}
		result.WriteString("\n")
	}

	// 其他工具
	if len(otherTools) > 0 {
		result.WriteString("### 其他工具\n")
		for _, t := range otherTools {
			result.WriteString(t + "\n")
		}
		result.WriteString("\n")
	}

	// 通用指导
	result.WriteString("### 工具调用原则\n")
	result.WriteString("1. 需要实时数据时，必须调用工具，不要编造数据\n")
	result.WriteString("2. 搜索类工具优先用于获取最新信息\n")
	result.WriteString("3. 工具返回结果后再组织回答\n")

	return result.String()
}

func (b *ExpertAgentBuilder) requiresMarketCodeArguments(config *models.AgentConfig) bool {
	if config == nil || len(config.Tools) == 0 {
		return false
	}
	hasKline := false
	hasRealtime := false
	for _, name := range config.Tools {
		switch name {
		case "get_kline_data":
			hasKline = true
		case "get_stock_realtime":
			hasRealtime = true
		}
	}
	return hasKline || hasRealtime
}

func (b *ExpertAgentBuilder) hasTool(config *models.AgentConfig, name string) bool {
	if config == nil || len(config.Tools) == 0 {
		return false
	}
	for _, toolName := range config.Tools {
		if toolName == name {
			return true
		}
	}
	return false
}

func preferredIndexCodeByStock(symbol string) string {
	code := strings.ToLower(strings.TrimSpace(symbol))
	switch {
	case strings.HasPrefix(code, "sh"):
		return "sh000001"
	case strings.HasPrefix(code, "sz"), strings.HasPrefix(code, "bj"):
		return "sz399001"
	default:
		return "sh000001"
	}
}
