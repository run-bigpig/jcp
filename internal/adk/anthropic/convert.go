package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/run-bigpig/jcp/internal/logger"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var convertLog = logger.New("anthropic:convert")

// toToolResultContent 将函数返回值转换为 Anthropic tool_result.content。
// Anthropic 要求 content 为字符串或内容块数组，这里统一归一为字符串 JSON。
func toToolResultContent(resp any) (json.RawMessage, error) {
	if s, ok := resp.(string); ok {
		b, err := json.Marshal(s)
		return b, err
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(b))
}

// extractTextFromContent 提取 genai.Content 中的纯文本
func extractTextFromContent(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var texts []string
	for _, part := range content.Parts {
		if part.Text != "" && !part.Thought {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}

// toAnthropicRequest 将 ADK LLMRequest 转换为 Anthropic Messages 请求
func toAnthropicRequest(req *model.LLMRequest, modelName string) (*MessagesRequest, error) {
	ar := &MessagesRequest{
		Model:     modelName,
		MaxTokens: 4096, // Anthropic 要求必须设置
	}

	// 系统指令 → 顶层 system 字段
	if req.Config != nil && req.Config.SystemInstruction != nil {
		ar.System = extractTextFromContent(req.Config.SystemInstruction)
	}

	// 转换消息
	msgs, err := toAnthropicMessages(req.Contents)
	if err != nil {
		return nil, err
	}
	ar.Messages = msgs

	// 转换工具
	if req.Config != nil && len(req.Config.Tools) > 0 {
		tools, err := convertTools(req.Config.Tools)
		if err != nil {
			return nil, err
		}
		ar.Tools = tools
	}

	// 应用配置
	if req.Config != nil {
		if req.Config.Temperature != nil {
			t := float64(*req.Config.Temperature)
			ar.Temperature = &t
		}
		if req.Config.MaxOutputTokens > 0 {
			ar.MaxTokens = int(req.Config.MaxOutputTokens)
		}
		if req.Config.TopP != nil {
			p := float64(*req.Config.TopP)
			ar.TopP = &p
		}
		if len(req.Config.StopSequences) > 0 {
			ar.StopSequences = req.Config.StopSequences
		}
	}

	return ar, nil
}

// toAnthropicMessages 将 genai.Content 列表转换为 Anthropic messages
func toAnthropicMessages(contents []*genai.Content) ([]Message, error) {
	var msgs []Message

	for _, content := range contents {
		if content == nil {
			continue
		}

		role := "user"
		if content.Role == "model" {
			role = "assistant"
		}

		var blocks []ContentBlock

		for _, part := range content.Parts {
			// 跳过 thought parts（不回传给 API）
			if part.Thought {
				continue
			}

			// 文本
			if part.Text != "" {
				blocks = append(blocks, ContentBlock{
					Type: "text",
					Text: part.Text,
				})
			}

			// 函数调用 → tool_use
			if part.FunctionCall != nil {
				inputJSON, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return nil, fmt.Errorf("marshal function call args: %w", err)
				}
				blocks = append(blocks, ContentBlock{
					Type:  "tool_use",
					ID:    part.FunctionCall.ID,
					Name:  part.FunctionCall.Name,
					Input: inputJSON,
				})
			}

			// 函数响应 → tool_result
			if part.FunctionResponse != nil {
				contentJSON, err := toToolResultContent(part.FunctionResponse.Response)
				if err != nil {
					return nil, fmt.Errorf("marshal function response: %w", err)
				}
				blocks = append(blocks, ContentBlock{
					Type:       "tool_result",
					ToolUseID:  part.FunctionResponse.ID,
					RawContent: contentJSON,
				})
			}
		}

		if len(blocks) == 0 {
			continue
		}

		// Anthropic 要求 user/assistant 交替，合并相同 role
		if len(msgs) > 0 && msgs[len(msgs)-1].Role == role {
			msgs[len(msgs)-1].Content = append(msgs[len(msgs)-1].Content, blocks...)
		} else {
			msgs = append(msgs, Message{Role: role, Content: blocks})
		}
	}

	return msgs, nil
}

// convertTools 将 genai.Tool 转换为 Anthropic Tool
func convertTools(genaiTools []*genai.Tool) ([]Tool, error) {
	var tools []Tool
	for _, gt := range genaiTools {
		if gt == nil {
			continue
		}
		for _, fd := range gt.FunctionDeclarations {
			schema := fd.ParametersJsonSchema
			if schema == nil {
				schema = fd.Parameters
			}
			if schema == nil {
				return nil, fmt.Errorf("parameters is nil for tool %s", fd.Name)
			}
			schemaJSON, err := json.Marshal(schema)
			if err != nil {
				return nil, fmt.Errorf("marshal tool schema: %w", err)
			}
			tools = append(tools, Tool{
				Name:        fd.Name,
				Description: fd.Description,
				InputSchema: schemaJSON,
			})
		}
	}
	return tools, nil
}

// convertAnthropicResponse 将 Anthropic 响应转换为 ADK LLMResponse
func convertAnthropicResponse(resp *MessagesResponse) (*model.LLMResponse, error) {
	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				content.Parts = append(content.Parts, &genai.Part{Text: block.Text})
			}
		case "thinking":
			if block.Thinking != "" {
				content.Parts = append(content.Parts, &genai.Part{Text: block.Thinking, Thought: true})
			}
		case "tool_use":
			args := make(map[string]any)
			if len(block.Input) > 0 {
				if err := json.Unmarshal(block.Input, &args); err != nil {
					convertLog.Warn("解析 tool_use input 失败: %v", err)
				}
			}
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   block.ID,
					Name: block.Name,
					Args: args,
				},
			})
		}
	}

	return &model.LLMResponse{
		Content:       content,
		UsageMetadata: convertUsage(&resp.Usage),
		FinishReason:  convertStopReason(resp.StopReason),
		TurnComplete:  true,
	}, nil
}

// convertUsage 转换 token 用量
func convertUsage(u *Usage) *genai.GenerateContentResponseUsageMetadata {
	if u == nil {
		return nil
	}
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:     int32(u.InputTokens),
		CandidatesTokenCount: int32(u.OutputTokens),
		TotalTokenCount:      int32(u.InputTokens + u.OutputTokens),
	}
}

// convertStopReason 转换停止原因
func convertStopReason(reason string) genai.FinishReason {
	switch reason {
	case "end_turn", "stop_sequence":
		return genai.FinishReasonStop
	case "max_tokens":
		return genai.FinishReasonMaxTokens
	case "tool_use":
		return genai.FinishReasonStop
	default:
		return genai.FinishReasonUnspecified
	}
}
