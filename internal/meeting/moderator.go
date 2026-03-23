package meeting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/models"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Moderator 小韭菜 Agent
type Moderator struct {
	llm model.LLM
}

// NewModerator 创建小韭菜
func NewModerator(llm model.LLM) *Moderator {
	return &Moderator{llm: llm}
}

// ModeratorDecision 小韭菜决策结果
type ModeratorDecision struct {
	Intent   string   `json:"intent"`
	Selected []string `json:"selected"`
	Topic    string   `json:"topic"`
	Opening  string   `json:"opening"`
}

// DiscussionEntry 讨论条目
type DiscussionEntry struct {
	Round     int    `json:"round"`
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
	Role      string `json:"role"`
	Content   string `json:"content"`
}

// Analyze 分析用户意图并选择专家
func (m *Moderator) Analyze(ctx context.Context, stock *models.Stock, query string, agents []models.AgentConfig) (*ModeratorDecision, error) {
	prompt := m.buildAnalyzePrompt(stock, query, agents)
	content, err := m.generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("moderator analyze error: %w", err)
	}
	return m.parseDecision(content)
}

// Summarize 总结讨论并给出结论
func (m *Moderator) Summarize(ctx context.Context, stock *models.Stock, query string, history []DiscussionEntry) (string, error) {
	prompt := m.buildSummarizePrompt(stock, query, history, "")
	return m.generate(ctx, prompt)
}

// SummarizeWithContext 总结讨论并结合补充数据给出结论
func (m *Moderator) SummarizeWithContext(ctx context.Context, stock *models.Stock, query string, history []DiscussionEntry, extraContext string) (string, error) {
	prompt := m.buildSummarizePrompt(stock, query, history, extraContext)
	return m.generate(ctx, prompt)
}

// generate 调用 LLM 生成内容
func (m *Moderator) generate(ctx context.Context, prompt string) (string, error) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{genai.NewPartFromText(prompt)}},
		},
	}

	var result strings.Builder
	for resp, err := range m.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp != nil && resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part.Thought {
					continue
				}
				if part.Text != "" {
					result.WriteString(part.Text)
				}
			}
		}
	}
	return result.String(), nil
}

// buildAnalyzePrompt 构建意图分析 Prompt
func (m *Moderator) buildAnalyzePrompt(stock *models.Stock, query string, agents []models.AgentConfig) string {
	var sb strings.Builder
	sb.WriteString("你是「财经会议室」的小韭菜，负责组织专家讨论。\n\n")
	sb.WriteString("## 当前股票\n")
	sb.WriteString(fmt.Sprintf("%s (%s)，现价 %.2f，涨跌幅 %.2f%%\n\n",
		stock.Name, stock.Symbol, stock.Price, stock.ChangePercent))
	sb.WriteString("## 老韭菜问题\n")
	sb.WriteString(query + "\n\n")
	sb.WriteString("## 可邀请的专家\n")
	for _, a := range agents {
		sb.WriteString(fmt.Sprintf("- %s（ID: %s）：%s\n", a.Name, a.ID, a.Role))
	}
	sb.WriteString("\n## 你的任务\n")
	sb.WriteString("1. 分析老韭菜问题的核心意图\n")
	sb.WriteString("2. 选择 2-4 位最相关的专家（简单问题选2位，常规选3位，复杂或高风险问题选4位）\n")
	sb.WriteString("3. 生成讨论议题和开场白\n\n")
	sb.WriteString("## 选人约束\n")
	sb.WriteString("1. 涉及买卖/仓位/止损建议时，必须包含至少 1 位风控视角专家\n")
	sb.WriteString("2. 涉及短线时点/盘中节奏时，至少包含技术面或资金面中的 1 位\n")
	sb.WriteString("3. 当问题涉及中长期或估值逻辑时，补充基本面/估值/政策/舆情/异动中的至少 1 位\n")
	sb.WriteString("4. 组合风格优先“1-2 位激进 + 其余稳健”，避免同质化\n")
	sb.WriteString("5. 避免固定组合，在满足相关性的前提下优先引入不同视角\n\n")
	sb.WriteString("## 输出格式（仅输出JSON）\n")
	sb.WriteString(`{"intent":"意图","selected":["id1"],"topic":"议题","opening":"开场白"}`)
	return sb.String()
}

// buildSummarizePrompt 构建总结 Prompt
func (m *Moderator) buildSummarizePrompt(stock *models.Stock, query string, history []DiscussionEntry, extraContext string) string {
	var sb strings.Builder
	sb.WriteString("你是会议小韭菜，请总结讨论并给老韭菜结论。\n\n")
	sb.WriteString(fmt.Sprintf("## 股票：%s (%s)\n\n", stock.Name, stock.Symbol))
	sb.WriteString("## 老韭菜问题\n")
	sb.WriteString(query + "\n\n")
	if strings.TrimSpace(extraContext) != "" {
		sb.WriteString("## 补充数据\n")
		sb.WriteString(extraContext + "\n\n")
	}
	sb.WriteString("## 讨论记录\n")
	for _, e := range history {
		sb.WriteString(fmt.Sprintf("【%s（%s）】\n%s\n\n", e.AgentName, e.Role, e.Content))
	}
	sb.WriteString("## 输出要求\n")
	sb.WriteString("请按 Markdown 输出，严格使用以下结构（标题必须单独一行，顺序固定，每个标题只出现一次）：\n")
	sb.WriteString("## 结论\n")
	sb.WriteString("> 这里写结论（仅“结论”段允许使用引用格式）\n")
	sb.WriteString("## 理由\n")
	sb.WriteString("## 触发与风控\n")
	sb.WriteString("## 失效条件\n")
	sb.WriteString("不要输出角标 [1][2]、不要输出“参考依据”区块，不要原样堆砌字段名。\n")
	sb.WriteString("不要使用代码块、行内代码、表格。\n")
	sb.WriteString("不要输出任何列表编号或混合编号（禁止 1) / (1) / 1. / 一、）。\n")
	sb.WriteString("小标题必须单独一行，不要把“标题：正文”写在同一行。\n")
	sb.WriteString("重点只用 **加粗**，不要使用除“结论段引用”之外的引用格式。\n")
	sb.WriteString("四段内容前后必须一致：若“结论”偏观望/持有/减仓，则“触发与风控”不得出现无条件买入或加仓。\n")
	sb.WriteString("内容必须覆盖：核心结论、综合建议与触发条件、风险与失效条件。\n")
	sb.WriteString("请融合专家观点并去重，避免重复段落。\n\n")
	sb.WriteString("控制在 220-420 字，严禁编造数据；缺失数据请明确写“未获取到”。")
	return sb.String()
}

// parseDecision 解析小韭菜决策 JSON（增强健壮性）
func (m *Moderator) parseDecision(content string) (*ModeratorDecision, error) {
	content = strings.TrimSpace(content)

	// 尝试多种方式提取 JSON
	jsonStr := m.extractJSON(content)
	if jsonStr == "" {
		return nil, fmt.Errorf("无法从响应中提取 JSON: %s", truncateString(content, 200))
	}

	var decision ModeratorDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w, 原文: %s", err, truncateString(jsonStr, 200))
	}

	// 验证必要字段
	if len(decision.Selected) == 0 {
		return nil, fmt.Errorf("小韭菜未选择任何专家")
	}

	return &decision, nil
}

// extractJSON 从文本中提取 JSON 对象
func (m *Moderator) extractJSON(content string) string {
	// 方法1: 尝试直接解析整个内容
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
		return content
	}

	// 方法2: 查找 ```json 代码块
	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// 方法3: 查找 ``` 代码块
	if idx := strings.Index(content, "```"); idx != -1 {
		start := idx + 3
		// 跳过可能的语言标识
		if newline := strings.Index(content[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		if end := strings.Index(content[start:], "```"); end != -1 {
			extracted := strings.TrimSpace(content[start : start+end])
			if strings.HasPrefix(extracted, "{") {
				return extracted
			}
		}
	}

	// 方法4: 查找第一个完整的 JSON 对象（匹配括号）
	start := strings.Index(content, "{")
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escape := false

	for i := start; i < len(content); i++ {
		c := content[i]

		if escape {
			escape = false
			continue
		}

		if c == '\\' && inString {
			escape = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return content[start : i+1]
			}
		}
	}

	// 方法5: 回退到简单的首尾匹配
	end := strings.LastIndex(content, "}")
	if end > start {
		return content[start : end+1]
	}

	return ""
}

// truncateString 截断字符串用于日志输出
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
