package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// Summarizer 摘要生成器接口
type Summarizer interface {
	SummarizeRounds(ctx context.Context, rounds []RoundMemory) (string, error)
	ExtractFacts(ctx context.Context, content, agentName string) ([]MemoryEntry, error)
	ExtractKeyPoints(ctx context.Context, discussions []DiscussionInput) ([]string, error)
}

// DiscussionInput 讨论输入（用于关键点提取）
type DiscussionInput struct {
	AgentName string
	Role      string
	Content   string
}

// LLMSummarizer 基于 LLM 的摘要生成器
type LLMSummarizer struct {
	llm       model.LLM
	tokenizer Tokenizer
}

// NewLLMSummarizer 创建 LLM 摘要生成器
func NewLLMSummarizer(llm model.LLM, tokenizer Tokenizer) *LLMSummarizer {
	return &LLMSummarizer{
		llm:       llm,
		tokenizer: tokenizer,
	}
}

// generate 调用 LLM 生成内容
func (s *LLMSummarizer) generate(ctx context.Context, prompt string) (string, error) {
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: prompt}},
			},
		},
	}

	var result string
	for resp, err := range s.llm.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp != nil && resp.Content != nil {
			for _, part := range resp.Content.Parts {
				if part.Text != "" {
					result += part.Text
				}
			}
		}
	}
	return result, nil
}

// SummarizeRounds 压缩多轮讨论为摘要
func (s *LLMSummarizer) SummarizeRounds(ctx context.Context, rounds []RoundMemory) (string, error) {
	if len(rounds) == 0 {
		return "", nil
	}

	prompt := s.buildSummarizePrompt(rounds)
	return s.generate(ctx, prompt)
}

func (s *LLMSummarizer) buildSummarizePrompt(rounds []RoundMemory) string {
	var sb strings.Builder
	sb.WriteString("请将以下多轮股票讨论压缩为简洁摘要。\n\n")
	sb.WriteString("要求：\n")
	sb.WriteString("1. 保留关键结论和观点\n")
	sb.WriteString("2. 去除重复信息\n")
	sb.WriteString("3. 控制在150字以内\n\n")
	sb.WriteString("讨论记录：\n")

	for _, r := range rounds {
		sb.WriteString(fmt.Sprintf("【第%d轮】问题: %s\n", r.Round, r.Query))
		sb.WriteString(fmt.Sprintf("结论: %s\n\n", r.Consensus))
	}

	sb.WriteString("摘要：")
	return sb.String()
}

// ExtractFacts 从讨论内容中提取关键事实
func (s *LLMSummarizer) ExtractFacts(ctx context.Context, content, agentName string) ([]MemoryEntry, error) {
	prompt := s.buildExtractPrompt(content)
	result, err := s.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return s.parseFacts(result, agentName)
}

func (s *LLMSummarizer) buildExtractPrompt(content string) string {
	return fmt.Sprintf(`从以下讨论内容中提取关键事实（最多5条）。

内容：
%s

请以JSON数组格式输出，每个事实包含：
- content: 事实内容（简洁，不超过50字）
- type: 类型（fact/opinion/decision）
- weight: 重要性 0-1

只输出JSON数组，不要其他内容：`, content)
}

func (s *LLMSummarizer) parseFacts(jsonStr, source string) ([]MemoryEntry, error) {
	// 清理 JSON
	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	var raw []struct {
		Content string    `json:"content"`
		Type    string    `json:"type"`
		Weight  float64   `json:"weight"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse facts json error: %w", err)
	}

	now := time.Now().UnixMilli()
	entries := make([]MemoryEntry, 0, len(raw))
	for _, r := range raw {
		// 使用分词器提取关键词
		keywords := s.tokenizer.Extract(r.Content, 5)

		entries = append(entries, MemoryEntry{
			ID:        uuid.New().String(),
			Type:      EntryType(r.Type),
			Content:   r.Content,
			Source:    source,
			Keywords:  keywords,
			Timestamp: now,
			Weight:    r.Weight,
		})
	}
	return entries, nil
}

// ExtractKeyPoints 从讨论中智能提取关键点
func (s *LLMSummarizer) ExtractKeyPoints(ctx context.Context, discussions []DiscussionInput) ([]string, error) {
	if len(discussions) == 0 {
		return []string{}, nil
	}

	prompt := s.buildKeyPointsPrompt(discussions)
	result, err := s.generate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return s.parseKeyPoints(result), nil
}

func (s *LLMSummarizer) buildKeyPointsPrompt(discussions []DiscussionInput) string {
	var sb strings.Builder
	sb.WriteString("从以下专家讨论中提取核心观点，每位专家提取1-2个最重要的观点。\n\n")

	for _, d := range discussions {
		sb.WriteString(fmt.Sprintf("【%s（%s）】\n%s\n\n", d.AgentName, d.Role, d.Content))
	}

	sb.WriteString("要求：\n")
	sb.WriteString("1. 每条观点简洁明了，不超过30字\n")
	sb.WriteString("2. 保留具体数据和结论\n")
	sb.WriteString("3. 格式：专家名: 观点内容\n")
	sb.WriteString("4. 每行一条，直接输出，不要编号\n")
	return sb.String()
}

func (s *LLMSummarizer) parseKeyPoints(result string) []string {
	lines := strings.Split(strings.TrimSpace(result), "\n")
	points := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			points = append(points, line)
		}
	}
	return points
}
