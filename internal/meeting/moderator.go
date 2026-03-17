package meeting

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/adk/openai"
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
	Intent   string            `json:"intent"`
	Selected []string          `json:"selected"`
	Topic    string            `json:"topic"`
	Opening  string            `json:"opening"`
	Tasks    map[string]string `json:"tasks"` // 专家ID -> 专属分析任务
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
	content, err := retryRun(ctx, MaxAgentRetries, func() (string, error) {
		return m.generate(ctx, prompt)
	})
	if err != nil {
		return nil, fmt.Errorf("moderator analyze error: %w", err)
	}
	return m.parseDecision(content)
}

// Summarize 总结讨论并给出结论
func (m *Moderator) Summarize(ctx context.Context, stock *models.Stock, query string, history []DiscussionEntry) (string, error) {
	prompt := m.buildSummarizePrompt(stock, query, history)
	return retryRun(ctx, MaxAgentRetries, func() (string, error) {
		return m.generate(ctx, prompt)
	})
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
	// 过滤第三方工具调用标记后返回
	cleaned := openai.FilterVendorToolCallMarkers(result.String())
	if cleaned == "" {
		return "", fmt.Errorf("模型返回空内容")
	}
	return cleaned, nil
}

// buildAnalyzePrompt 构建意图分析 Prompt
func (m *Moderator) buildAnalyzePrompt(stock *models.Stock, query string, agents []models.AgentConfig) string {
	var sb strings.Builder
	sb.WriteString("你是「财经会议室」的小韭菜，负责组织专家讨论。\n\n")
	sb.WriteString("## 当前股票\n")
	fmt.Fprintf(&sb, "%s (%s)，现价 %.2f，涨跌幅 %.2f%%\n\n", stock.Name, stock.Symbol, stock.Price, stock.ChangePercent)
	sb.WriteString("## 老韭菜问题\n")
	sb.WriteString(query + "\n\n")
	sb.WriteString("## 可邀请的专家\n")
	for _, a := range agents {
		fmt.Fprintf(&sb, "- %s（ID: %s）：%s\n", a.Name, a.ID, a.Role)
	}
	sb.WriteString("\n## 你的任务\n")
	sb.WriteString("1. 分析老韭菜问题的核心意图\n")
	sb.WriteString(fmt.Sprintf("2. 除非用户特别约束专家数量,否则选择 1-%d 位最相关的专家\n", len(agents)))
	sb.WriteString("3. 为每位选中的专家制定一个明确的、与其专业匹配的分析任务（不要照搬用户原话，要根据专家角色拆解）\n")
	sb.WriteString("4. 生成讨论议题和开场白\n\n")
	sb.WriteString("## 输出格式（仅输出JSON）\n")
	sb.WriteString(`{"intent":"意图","selected":["id1","id2"],"tasks":{"id1":"该专家需要分析的具体问题","id2":"该专家需要分析的具体问题"},"topic":"议题","opening":"开场白"}`)
	return sb.String()
}

// buildSummarizePrompt 构建总结 Prompt
func (m *Moderator) buildSummarizePrompt(stock *models.Stock, query string, history []DiscussionEntry) string {
	var sb strings.Builder
	sb.WriteString("你是会议小韭菜，请总结讨论并给老韭菜结论。\n\n")
	fmt.Fprintf(&sb, "## 股票：%s (%s)\n\n", stock.Name, stock.Symbol)
	sb.WriteString("## 老韭菜问题\n")
	sb.WriteString(query + "\n\n")
	sb.WriteString("## 讨论记录\n")
	for _, e := range history {
		fmt.Fprintf(&sb, "【%s（%s）】\n%s\n\n", e.AgentName, e.Role, e.Content)
	}
	sb.WriteString("## 输出要求\n")
	sb.WriteString("1. 核心结论（直接回答老韭菜）\n")
	sb.WriteString("2. 各方观点摘要\n")
	sb.WriteString("3. 综合建议\n\n")
	sb.WriteString("控制在 300 字以内。")
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
