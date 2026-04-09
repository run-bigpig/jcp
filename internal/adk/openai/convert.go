package openai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/run-bigpig/jcp/internal/logger"
)

var convertLog = logger.New("openai:convert")

// 匹配第三方特殊工具调用格式
// 格式1: <vendor:tool_call> <invoke name="xxx"> <parameter name="yyy">zzz</parameter> </invoke> </vendor:tool_call>
var vendorToolCallStartRegex = regexp.MustCompile(`<(\w+):tool_call>`)
var invokeRegex = regexp.MustCompile(`(?s)<invoke\s+name="([^"]+)">\s*(.*?)\s*</invoke>`)
var paramRegex = regexp.MustCompile(`(?s)<parameter\s+name="([^"]+)">(.*?)</parameter>`)

// 格式2: <tool_call_begin>tool_name <param name="xxx">yyy</param> </tool_call_end>
var toolCallBeginRegex = regexp.MustCompile(`(?s)<tool_call_begin>\s*(\w+)\s*(.*?)\s*</tool_call_end>`)
var paramAltRegex = regexp.MustCompile(`(?s)<param\s+name="([^"]+)">(.*?)</param>`)

// 格式3: <tool_call> <tool name="xxx"> <param name="yyy">zzz</param> </tool> </tool_call>
var toolCallWrapRegex = regexp.MustCompile(`(?s)<tool_call>\s*(.*?)\s*</tool_call>`)
var toolTagRegex = regexp.MustCompile(`(?s)<tool\s+name="([^"]+)">\s*(.*?)\s*</tool>`)

// VendorToolCall 第三方工具调用解析结果
type VendorToolCall struct {
	Name string
	Args map[string]any
}

// FilterVendorToolCallMarkers 过滤文本中的第三方工具调用标记（导出供外部使用）
func FilterVendorToolCallMarkers(text string) string {
	_, cleaned := parseVendorToolCalls(text)
	return cleaned
}

// parseVendorToolCalls 解析文本中的第三方工具调用标记
// 返回解析出的工具调用列表和清理后的文本
func parseVendorToolCalls(text string) ([]VendorToolCall, string) {
	if text == "" {
		return nil, text
	}

	var toolCalls []VendorToolCall
	cleanedText := text

	// 查找所有 vendor:tool_call 开始标签
	startMatches := vendorToolCallStartRegex.FindAllStringSubmatchIndex(text, -1)
	for _, match := range startMatches {
		if len(match) < 4 {
			continue
		}
		// match[0]:match[1] 是整个匹配，match[2]:match[3] 是 vendor 名称
		vendor := text[match[2]:match[3]]
		startPos := match[0]
		endTag := "</" + vendor + ":tool_call>"

		// 查找对应的结束标签
		endPos := strings.Index(text[match[1]:], endTag)
		if endPos == -1 {
			continue
		}
		endPos += match[1]

		// 提取内部内容
		innerContent := text[match[1]:endPos]
		fullMatch := text[startPos : endPos+len(endTag)]

		// 解析 invoke 标签
		invokeMatches := invokeRegex.FindAllStringSubmatch(innerContent, -1)
		for _, invokeMatch := range invokeMatches {
			if len(invokeMatch) < 3 {
				continue
			}
			toolName := invokeMatch[1]
			paramsContent := invokeMatch[2]

			// 解析参数
			args := make(map[string]any)
			paramMatches := paramRegex.FindAllStringSubmatch(paramsContent, -1)
			for _, paramMatch := range paramMatches {
				if len(paramMatch) >= 3 {
					args[paramMatch[1]] = paramMatch[2]
				}
			}

			toolCalls = append(toolCalls, VendorToolCall{
				Name: toolName,
				Args: args,
			})
		}

		// 从文本中移除已解析的工具调用块
		cleanedText = strings.Replace(cleanedText, fullMatch, "", 1)
	}

	// 格式2: <tool_call_begin>tool_name <param name="xxx">yyy</param> </tool_call_end>
	beginMatches := toolCallBeginRegex.FindAllStringSubmatch(cleanedText, -1)
	for _, match := range beginMatches {
		if len(match) < 3 {
			continue
		}
		toolName := match[1]
		paramsContent := match[2]

		// 解析参数
		args := make(map[string]any)
		paramMatches := paramAltRegex.FindAllStringSubmatch(paramsContent, -1)
		for _, paramMatch := range paramMatches {
			if len(paramMatch) >= 3 {
				// 去除参数值两端的引号
				val := strings.Trim(paramMatch[2], "\"")
				args[paramMatch[1]] = val
			}
		}

		toolCalls = append(toolCalls, VendorToolCall{
			Name: toolName,
			Args: args,
		})

		// 从文本中移除已解析的工具调用块
		cleanedText = strings.Replace(cleanedText, match[0], "", 1)
	}

	// 格式3: <tool_call> <tool name="xxx"> <param name="yyy">zzz</param> </tool> </tool_call>
	wrapMatches := toolCallWrapRegex.FindAllStringSubmatch(cleanedText, -1)
	for _, match := range wrapMatches {
		if len(match) < 2 {
			continue
		}
		innerContent := match[1]

		// 解析多个 tool 标签
		toolMatches := toolTagRegex.FindAllStringSubmatch(innerContent, -1)
		for _, toolMatch := range toolMatches {
			if len(toolMatch) < 3 {
				continue
			}
			toolName := toolMatch[1]
			paramsContent := toolMatch[2]

			// 解析参数
			args := make(map[string]any)
			paramMatches := paramAltRegex.FindAllStringSubmatch(paramsContent, -1)
			for _, paramMatch := range paramMatches {
				if len(paramMatch) >= 3 {
					val := strings.Trim(paramMatch[2], "\"")
					args[paramMatch[1]] = val
				}
			}

			toolCalls = append(toolCalls, VendorToolCall{
				Name: toolName,
				Args: args,
			})
		}

		// 从文本中移除已解析的工具调用块
		cleanedText = strings.Replace(cleanedText, match[0], "", 1)
	}

	return toolCalls, strings.TrimSpace(cleanedText)
}

// toOpenAIChatCompletionRequest 将 ADK 请求转换为 OpenAI 请求
func toOpenAIChatCompletionRequest(req *model.LLMRequest, modelName string, noSystemRole bool, tokenParamMode string) (openai.ChatCompletionRequest, error) {
	openaiMessages := make([]openai.ChatCompletionMessage, 0, len(req.Contents))
	for _, content := range req.Contents {
		msgs, err := toOpenAIChatCompletionMessage(content)
		if err != nil {
			return openai.ChatCompletionRequest{}, err
		}
		openaiMessages = append(openaiMessages, msgs...)
	}

	openaiReq := openai.ChatCompletionRequest{
		Model:    modelName,
		Messages: openaiMessages,
	}

	// 处理 thinking 配置
	if req.Config != nil && req.Config.ThinkingConfig != nil {
		switch req.Config.ThinkingConfig.ThinkingLevel {
		case genai.ThinkingLevelLow:
			openaiReq.ReasoningEffort = "low"
		case genai.ThinkingLevelHigh:
			openaiReq.ReasoningEffort = "high"
		default:
			openaiReq.ReasoningEffort = "medium"
		}
	}

	// 处理工具
	if req.Config != nil && len(req.Config.Tools) > 0 {
		tools, err := convertTools(req.Config.Tools)
		if err != nil {
			return openai.ChatCompletionRequest{}, err
		}
		openaiReq.Tools = tools
	}

	// 应用配置
	if req.Config != nil {
		if req.Config.Temperature != nil {
			openaiReq.Temperature = *req.Config.Temperature
		}
		if req.Config.MaxOutputTokens > 0 {
			switch ResolveTokenParamMode(modelName, tokenParamMode) {
			case TokenParamModeMaxCompletionTokens:
				openaiReq.MaxCompletionTokens = int(req.Config.MaxOutputTokens)
			default:
				openaiReq.MaxTokens = int(req.Config.MaxOutputTokens)
			}
		}
		if req.Config.TopP != nil {
			openaiReq.TopP = *req.Config.TopP
		}
		if len(req.Config.StopSequences) > 0 {
			openaiReq.Stop = req.Config.StopSequences
		}

		// 处理系统指令
		if req.Config.SystemInstruction != nil {
			systemText := extractTextFromContent(req.Config.SystemInstruction)
			if noSystemRole {
				// 不支持 system role，将系统指令注入到第一条 user 消息前面
				injected := false
				for i, msg := range openaiMessages {
					if msg.Role == openai.ChatMessageRoleUser {
						openaiMessages[i].Content = systemText + "\n\n" + msg.Content
						injected = true
						break
					}
				}
				if !injected {
					// 没有 user 消息，作为独立 user 消息插入
					userMsg := openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: systemText,
					}
					openaiMessages = append([]openai.ChatCompletionMessage{userMsg}, openaiMessages...)
				}
			} else {
				systemMsg := openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemText,
				}
				openaiMessages = append([]openai.ChatCompletionMessage{systemMsg}, openaiMessages...)
			}
			openaiReq.Messages = openaiMessages
		}

		// 处理 JSON 模式
		if req.Config.ResponseMIMEType == "application/json" {
			openaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		}
	}

	return openaiReq, nil
}

// toOpenAIChatCompletionMessage 将 genai.Content 转换为 OpenAI 消息
// 关键：处理 thinking 模型的 reasoning_content
func toOpenAIChatCompletionMessage(content *genai.Content) ([]openai.ChatCompletionMessage, error) {
	// 先处理 function response 消息
	toolRespMessages := make([]openai.ChatCompletionMessage, 0)
	skipIdx := 0
	for idx, part := range content.Parts {
		if part.FunctionResponse != nil {
			openaiMsg := openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: part.FunctionResponse.ID,
			}
			responseJSON, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function response: %w", err)
			}
			openaiMsg.Content = string(responseJSON)
			toolRespMessages = append(toolRespMessages, openaiMsg)
			skipIdx = idx + 1
			continue
		}
	}

	parts := content.Parts[skipIdx:]
	if len(parts) == 0 {
		return toolRespMessages, nil
	}

	openaiMsg := openai.ChatCompletionMessage{
		Role: convertRoleToOpenAI(content.Role),
	}

	// 收集各类内容
	var textContent string
	var reasoningContent string
	var toolCalls []openai.ToolCall

	for _, part := range parts {
		// 处理 thinking/reasoning 内容
		if part.Thought && part.Text != "" {
			reasoningContent += part.Text
			continue
		}

		// 处理普通文本
		if part.Text != "" {
			textContent += part.Text
		}

		// 处理函数调用
		if part.FunctionCall != nil {
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function args: %w", err)
			}
			toolCall := openai.ToolCall{
				ID:   part.FunctionCall.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      part.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	// 设置消息内容
	if textContent != "" {
		openaiMsg.Content = textContent
	}

	// 关键：设置 reasoning_content 用于 thinking 模型
	if reasoningContent != "" {
		openaiMsg.ReasoningContent = reasoningContent
	}

	if len(toolCalls) > 0 {
		openaiMsg.ToolCalls = toolCalls
	}

	return append(toolRespMessages, openaiMsg), nil
}

// convertRoleToOpenAI 转换角色
func convertRoleToOpenAI(role string) string {
	switch role {
	case "user":
		return openai.ChatMessageRoleUser
	case "model":
		return openai.ChatMessageRoleAssistant
	case "system":
		return openai.ChatMessageRoleSystem
	default:
		return openai.ChatMessageRoleUser
	}
}

// extractTextFromContent 提取文本内容
func extractTextFromContent(content *genai.Content) string {
	if content == nil {
		return ""
	}
	var texts []string
	for _, part := range content.Parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	if len(texts) == 0 {
		return ""
	}
	result := texts[0]
	for i := 1; i < len(texts); i++ {
		result += "\n" + texts[i]
	}
	return result
}

// convertTools 转换工具定义
func convertTools(genaiTools []*genai.Tool) ([]openai.Tool, error) {
	var openaiTools []openai.Tool

	for _, genaiTool := range genaiTools {
		if genaiTool == nil {
			continue
		}

		for _, funcDecl := range genaiTool.FunctionDeclarations {
			openaiTool := openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        funcDecl.Name,
					Description: funcDecl.Description,
					Parameters:  funcDecl.ParametersJsonSchema,
				},
			}
			if openaiTool.Function.Parameters == nil {
				openaiTool.Function.Parameters = funcDecl.Parameters
			}
			if openaiTool.Function.Parameters == nil {
				return nil, fmt.Errorf("parameters is nil for tool %s", funcDecl.Name)
			}
			openaiTools = append(openaiTools, openaiTool)
		}
	}

	return openaiTools, nil
}

// convertChatCompletionResponse 转换 OpenAI 响应
func convertChatCompletionResponse(resp *openai.ChatCompletionResponse) (*model.LLMResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, ErrNoChoicesInResponse
	}

	choice := resp.Choices[0]
	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}

	// 处理 reasoning_content (thinking 模型)
	if choice.Message.ReasoningContent != "" {
		content.Parts = append(content.Parts, &genai.Part{
			Text:    choice.Message.ReasoningContent,
			Thought: true,
		})
	}

	// 处理普通内容，解析第三方特殊工具调用标记
	if choice.Message.Content != "" {
		vendorCalls, cleanedText := parseVendorToolCalls(choice.Message.Content)
		// 解析 <think> 标签并映射到 Thought
		for _, seg := range splitThinkTaggedText(cleanedText) {
			content.Parts = append(content.Parts, &genai.Part{
				Text:    seg.Text,
				Thought: seg.Thought,
			})
		}
		// 将第三方工具调用转换为 FunctionCall
		for i, vc := range vendorCalls {
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   fmt.Sprintf("vendor_call_%d", i),
					Name: vc.Name,
					Args: vc.Args,
				},
			})
		}
	}

	// 处理标准 OpenAI 工具调用
	for _, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type == openai.ToolTypeFunction {
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   toolCall.ID,
					Name: toolCall.Function.Name,
					Args: parseJSONArgs(toolCall.Function.Arguments),
				},
			})
		}
	}

	// 兼容旧版 function_call 字段
	if choice.Message.FunctionCall != nil && choice.Message.FunctionCall.Name != "" {
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   "legacy_function_call",
				Name: choice.Message.FunctionCall.Name,
				Args: parseJSONArgs(choice.Message.FunctionCall.Arguments),
			},
		})
	}

	// 处理 usage
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	if resp.Usage.TotalTokens > 0 {
		usageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.PromptTokens),
			CandidatesTokenCount: int32(resp.Usage.CompletionTokens),
			TotalTokenCount:      int32(resp.Usage.TotalTokens),
		}
	}

	return &model.LLMResponse{
		Content:       content,
		UsageMetadata: usageMetadata,
		FinishReason:  convertFinishReason(string(choice.FinishReason)),
		TurnComplete:  true,
	}, nil
}

// convertFinishReason 转换结束原因
func convertFinishReason(reason string) genai.FinishReason {
	switch reason {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	case "tool_calls", "function_call":
		return genai.FinishReasonStop
	case "content_filter":
		return genai.FinishReasonSafety
	default:
		return genai.FinishReasonUnspecified
	}
}

// parseJSONArgs 解析 JSON 参数
func parseJSONArgs(argsJSON string) map[string]any {
	if argsJSON == "" {
		return make(map[string]any)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		convertLog.Warn("解析工具调用参数失败: %v, 原始内容: %s", err, argsJSON)
		return make(map[string]any)
	}
	return args
}
