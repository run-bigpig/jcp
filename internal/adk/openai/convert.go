package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// toOpenAIChatCompletionRequest 将 ADK 请求转换为 OpenAI 请求
func toOpenAIChatCompletionRequest(req *model.LLMRequest, modelName string) (openai.ChatCompletionRequest, error) {
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
		skipSampling := shouldSkipSamplingParams(modelName)
		if req.Config.Temperature != nil && !skipSampling {
			openaiReq.Temperature = *req.Config.Temperature
		}
		if req.Config.MaxOutputTokens > 0 {
			openaiReq.MaxCompletionTokens = int(req.Config.MaxOutputTokens)
		}
		if req.Config.TopP != nil && !skipSampling {
			openaiReq.TopP = *req.Config.TopP
		}
		if len(req.Config.StopSequences) > 0 {
			openaiReq.Stop = req.Config.StopSequences
		}

		// 处理系统指令
		if req.Config.SystemInstruction != nil {
			systemMsg := openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: extractTextFromContent(req.Config.SystemInstruction),
			}
			openaiMessages = append([]openai.ChatCompletionMessage{systemMsg}, openaiMessages...)
			openaiReq.Messages = openaiMessages
		}

		// 处理 JSON 模式
		if req.Config.ResponseMIMEType == "application/json" {
			openaiReq.ResponseFormat = &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			}
		}
	}

	// 推理模型限制：强制采样参数为 1，避免本地校验失败
	if isReasoningModel(modelName) {
		if openaiReq.Temperature != 1 {
			openaiReq.Temperature = 1
		}
		if openaiReq.TopP != 1 {
			openaiReq.TopP = 1
		}
		if openaiReq.N != 1 {
			openaiReq.N = 1
		}
		openaiReq.PresencePenalty = 0
		openaiReq.FrequencyPenalty = 0
	}

	return openaiReq, nil
}

func shouldSkipSamplingParams(modelName string) bool {
	return isReasoningModel(modelName)
}

func isReasoningModel(modelName string) bool {
	name := strings.ToLower(modelName)
	if strings.HasPrefix(name, "o1") || strings.HasPrefix(name, "o3") || strings.HasPrefix(name, "o4") {
		return true
	}
	if strings.HasPrefix(name, "gpt-5") || strings.HasPrefix(name, "gpt5") {
		return true
	}
	return false
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
			openaiTool.Function.Parameters = normalizeOpenAIToolParametersSchema(openaiTool.Function.Parameters)
			if openaiTool.Function.Parameters == nil {
				openaiTool.Function.Parameters = map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				}
			}
			openaiTools = append(openaiTools, openaiTool)
		}
	}

	return openaiTools, nil
}

func normalizeOpenAIToolParametersSchema(params any) any {
	if params == nil {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	raw, err := json.Marshal(params)
	if err != nil {
		return params
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil || schema == nil {
		return params
	}

	if len(schema) == 0 {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}
	}

	if _, ok := schema["type"]; !ok {
		schema["type"] = "object"
	}
	if t, ok := schema["type"].(string); ok && t == "object" {
		if _, ok := schema["properties"]; !ok {
			schema["properties"] = map[string]any{}
		}
	}

	return schema
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

	// 处理普通内容
	if choice.Message.Content != "" {
		content.Parts = append(content.Parts, &genai.Part{Text: choice.Message.Content})
	}

	// 处理工具调用
	for i, toolCall := range choice.Message.ToolCalls {
		if toolCall.Type == openai.ToolTypeFunction {
			callID := ensureFunctionCallID(toolCall.ID, toolCall.Function.Name, i)
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   callID,
					Name: toolCall.Function.Name,
					Args: parseJSONArgs(toolCall.Function.Arguments),
				},
			})
		}
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
	text := strings.TrimSpace(argsJSON)
	if text == "" {
		return make(map[string]any)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(text), &args); err == nil {
		return args
	}

	// 兼容部分网关返回被再次 JSON 编码的字符串参数
	var wrapped string
	if err := json.Unmarshal([]byte(text), &wrapped); err == nil {
		wrapped = strings.TrimSpace(wrapped)
		if wrapped != "" && json.Unmarshal([]byte(wrapped), &args) == nil {
			return args
		}
	}

	log.Warn("parse function args failed, raw=%s", normalizeLogText(text, 200))
	return make(map[string]any)
}
