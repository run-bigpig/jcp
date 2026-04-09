package openai

import (
	"encoding/json"
	"fmt"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

// toResponsesRequest 将 ADK 请求转换为 Responses API 请求
func toResponsesRequest(req *model.LLMRequest, modelName string, noSystemRole bool) (CreateResponseRequest, error) {
	// 转换 input 消息
	inputItems, err := toResponsesInputItems(req.Contents)
	if err != nil {
		return CreateResponseRequest{}, err
	}

	apiReq := CreateResponseRequest{
		Model: modelName,
		Input: inputItems,
	}

	if req.Config == nil {
		return apiReq, nil
	}

	// 处理系统指令
	if req.Config.SystemInstruction != nil {
		systemText := extractTextFromContent(req.Config.SystemInstruction)
		if noSystemRole {
			// 不支持 instructions 字段，将系统指令注入到第一条 user input 前面
			injected := false
			for i, item := range inputItems {
				if item.Role == "user" {
					if s, ok := item.Content.(string); ok {
						inputItems[i].Content = systemText + "\n\n" + s
					} else {
						inputItems[i].Content = systemText
					}
					injected = true
					break
				}
			}
			if !injected {
				inputItems = append([]ResponsesInputItem{{
					Role:    "user",
					Content: systemText,
				}}, inputItems...)
			}
			apiReq.Input = inputItems
		} else {
			apiReq.Instructions = systemText
		}
	}

	// 处理 thinking/reasoning 配置
	if req.Config.ThinkingConfig != nil {
		reasoning := &ResponsesReasoning{}
		switch req.Config.ThinkingConfig.ThinkingLevel {
		case genai.ThinkingLevelLow:
			reasoning.Effort = "low"
		case genai.ThinkingLevelHigh:
			reasoning.Effort = "high"
		default:
			reasoning.Effort = "medium"
		}
		apiReq.Reasoning = reasoning
	}

	// 转换工具定义
	if len(req.Config.Tools) > 0 {
		apiReq.Tools, err = convertResponsesTools(req.Config.Tools)
		if err != nil {
			return CreateResponseRequest{}, err
		}
	}

	// 应用生成参数
	if req.Config.Temperature != nil {
		t := float32(*req.Config.Temperature)
		apiReq.Temperature = &t
	}
	if req.Config.MaxOutputTokens > 0 {
		apiReq.MaxOutputTokens = int(req.Config.MaxOutputTokens)
	}
	if req.Config.TopP != nil {
		p := float32(*req.Config.TopP)
		apiReq.TopP = &p
	}
	if len(req.Config.StopSequences) > 0 {
		apiReq.Stop = req.Config.StopSequences
	}

	return apiReq, nil
}

// toResponsesInputItems 将 genai.Content 列表转换为 Responses API input
func toResponsesInputItems(contents []*genai.Content) ([]ResponsesInputItem, error) {
	var items []ResponsesInputItem

	for _, content := range contents {
		newItems, err := toResponsesInputItem(content)
		if err != nil {
			return nil, err
		}
		items = append(items, newItems...)
	}

	return items, nil
}

// toResponsesInputItem 将单个 genai.Content 转换为 Responses API input 项
func toResponsesInputItem(content *genai.Content) ([]ResponsesInputItem, error) {
	var items []ResponsesInputItem

	// 先处理 function response（工具调用结果）
	for _, part := range content.Parts {
		if part.FunctionResponse != nil {
			responseJSON, err := json.Marshal(part.FunctionResponse.Response)
			if err != nil {
				return nil, fmt.Errorf("序列化函数响应失败: %w", err)
			}
			items = append(items, ResponsesInputItem{
				Type:   "function_call_output",
				CallID: part.FunctionResponse.ID,
				Output: string(responseJSON),
			})
		}
	}

	// 收集文本、reasoning、函数调用
	var textContent string
	var toolCallItems []ResponsesInputItem

	for _, part := range content.Parts {
		if part.FunctionResponse != nil {
			continue // 已处理
		}
		if part.Text != "" && !part.Thought {
			textContent += part.Text
		}
		if part.FunctionCall != nil {
			argsJSON, err := json.Marshal(part.FunctionCall.Args)
			if err != nil {
				return nil, fmt.Errorf("序列化函数参数失败: %w", err)
			}
			toolCallItems = append(toolCallItems, ResponsesInputItem{
				Type:      "function_call",
				CallID:    part.FunctionCall.ID,
				Name:      part.FunctionCall.Name,
				Arguments: string(argsJSON),
			})
		}
	}

	// 构建普通消息
	role := convertRoleForResponses(content.Role)
	if textContent != "" {
		items = append(items, ResponsesInputItem{
			Role:    role,
			Content: textContent,
		})
	}

	// assistant 的工具调用作为独立 input 项
	items = append(items, toolCallItems...)

	return items, nil
}

// convertRoleForResponses 转换角色为 Responses API 格式
func convertRoleForResponses(role string) string {
	switch role {
	case "user":
		return "user"
	case "model":
		return "assistant"
	case "system":
		return "system"
	default:
		return "user"
	}
}

// convertResponsesTools 转换工具定义为 Responses API 扁平化格式
func convertResponsesTools(genaiTools []*genai.Tool) ([]ResponsesTool, error) {
	var tools []ResponsesTool
	for _, genaiTool := range genaiTools {
		if genaiTool == nil {
			continue
		}
		for _, funcDecl := range genaiTool.FunctionDeclarations {
			params := funcDecl.ParametersJsonSchema
			if params == nil {
				params = funcDecl.Parameters
			}
			params, err := normalizeResponsesToolSchema(params)
			if err != nil {
				return nil, fmt.Errorf("normalize responses tool schema %s: %w", funcDecl.Name, err)
			}
			tools = append(tools, ResponsesTool{
				Type:        "function",
				Name:        funcDecl.Name,
				Description: funcDecl.Description,
				Parameters:  params,
			})
		}
	}
	return tools, nil
}

func normalizeResponsesToolSchema(schema any) (any, error) {
	if schema == nil {
		return defaultResponsesToolSchema(), nil
	}

	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}
	if string(raw) == "null" {
		return defaultResponsesToolSchema(), nil
	}

	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}
	if normalized == nil {
		return defaultResponsesToolSchema(), nil
	}

	normalizeResponsesSchemaNode(normalized)
	if root, ok := normalized.(map[string]any); ok {
		ensureResponsesObjectSchema(root)
	}

	return normalized, nil
}

func defaultResponsesToolSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

func normalizeResponsesSchemaNode(node any) {
	switch typed := node.(type) {
	case map[string]any:
		ensureResponsesObjectSchema(typed)
		for key, value := range typed {
			if key == "additionalProperties" {
				if schemaMap, ok := value.(map[string]any); ok && isAlwaysFalseSchema(schemaMap) {
					typed[key] = false
					continue
				}
			}
			normalizeResponsesSchemaNode(value)
		}
	case []any:
		for _, item := range typed {
			normalizeResponsesSchemaNode(item)
		}
	}
}

func ensureResponsesObjectSchema(schema map[string]any) {
	if !isObjectSchemaType(schema["type"]) {
		return
	}
	if props, ok := schema["properties"]; !ok || props == nil {
		schema["properties"] = map[string]any{}
	}
}

func isObjectSchemaType(schemaType any) bool {
	switch typed := schemaType.(type) {
	case string:
		return typed == "object"
	case []any:
		for _, item := range typed {
			if item == "object" {
				return true
			}
		}
	}
	return false
}

func isAlwaysFalseSchema(schema map[string]any) bool {
	if len(schema) != 1 {
		return false
	}
	notSchema, ok := schema["not"].(map[string]any)
	return ok && len(notSchema) == 0
}

// convertResponsesResponse 将 Responses API 响应转换为 ADK LLMResponse
func convertResponsesResponse(resp *CreateResponseResponse) (*model.LLMResponse, error) {
	if len(resp.Output) == 0 {
		return nil, ErrNoChoicesInResponse
	}

	content := &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{},
	}

	for _, item := range resp.Output {
		switch item.Type {
		case "message":
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					// 解析第三方特殊工具调用标记
					vendorCalls, cleanedText := parseVendorToolCalls(part.Text)
					for _, seg := range splitThinkTaggedText(cleanedText) {
						content.Parts = append(content.Parts, &genai.Part{
							Text:    seg.Text,
							Thought: seg.Thought,
						})
					}
					for i, vc := range vendorCalls {
						content.Parts = append(content.Parts, &genai.Part{
							FunctionCall: &genai.FunctionCall{
								ID:   fmt.Sprintf("vendor_call_%d", i),
								Name: vc.Name,
								Args: vc.Args,
							},
						})
					}
				case "reasoning":
					content.Parts = append(content.Parts, &genai.Part{
						Text:    part.Text,
						Thought: true,
					})
				}
			}
		case "function_call":
			content.Parts = append(content.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   item.CallID,
					Name: item.Name,
					Args: parseJSONArgs(item.Arguments),
				},
			})
		}
	}

	// 处理 usage
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	if resp.Usage != nil {
		usageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.InputTokens),
			CandidatesTokenCount: int32(resp.Usage.OutputTokens),
			TotalTokenCount:      int32(resp.Usage.TotalTokens),
		}
	}

	return &model.LLMResponse{
		Content:       content,
		UsageMetadata: usageMetadata,
		FinishReason:  genai.FinishReasonStop,
		TurnComplete:  true,
	}, nil
}
