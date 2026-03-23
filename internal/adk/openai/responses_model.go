package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var _ model.LLM = &ResponsesModel{}

// HTTPDoer HTTP 客户端接口（与 go-openai 兼容）
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ResponsesModel 实现 model.LLM 接口，使用 OpenAI Responses API
type ResponsesModel struct {
	httpClient HTTPDoer
	baseURL    string
	apiKey     string
	modelName  string
}

// NewResponsesModel 创建 Responses API 模型
// apiKey 从工厂单独传入，因 go-openai ClientConfig.authToken 不可导出
func NewResponsesModel(modelName, apiKey, baseURL string, httpClient HTTPDoer) *ResponsesModel {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &ResponsesModel{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		modelName:  modelName,
	}
}

// Name 返回模型名称
func (r *ResponsesModel) Name() string {
	return r.modelName
}

// GenerateContent 实现 model.LLM 接口
func (r *ResponsesModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return r.generateStream(ctx, req)
	}
	return r.generate(ctx, req)
}

// responsesEndpoint 返回 Responses API 端点 URL
// baseURL 已由工厂层 normalizeOpenAIBaseURL 规范化，保证以 /v1 结尾
func (r *ResponsesModel) responsesEndpoint() string {
	return r.baseURL + "/responses"
}

// doRequest 发送 HTTP 请求到 Responses API
func (r *ResponsesModel) doRequest(ctx context.Context, body []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.responsesEndpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
	}
	return r.httpClient.Do(req)
}

// generate 非流式生成
func (r *ResponsesModel) generate(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		apiReq, err := toResponsesRequest(req, r.modelName)
		if err != nil {
			yield(nil, err)
			return
		}
		apiReq.Stream = false

		body, err := json.Marshal(apiReq)
		if err != nil {
			yield(nil, fmt.Errorf("序列化请求失败: %w", err))
			return
		}

		resp, err := r.doRequest(ctx, body, false)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			bodySnippet := normalizeLogText(string(respBody), 320)
			log.Error("responses request failed: model=%s status=%d body=%s", r.modelName, resp.StatusCode, bodySnippet)
			yield(nil, fmt.Errorf("Responses API 错误 (HTTP %d): %s", resp.StatusCode, bodySnippet))
			return
		}

		var apiResp CreateResponseResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			yield(nil, fmt.Errorf("解析响应失败: %w", err))
			return
		}

		llmResp, err := convertResponsesResponse(&apiResp)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(llmResp, nil)
	}
}

// generateStream 流式生成
func (r *ResponsesModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		apiReq, err := toResponsesRequest(req, r.modelName)
		if err != nil {
			yield(nil, err)
			return
		}
		apiReq.Stream = true

		body, err := json.Marshal(apiReq)
		if err != nil {
			yield(nil, fmt.Errorf("序列化请求失败: %w", err))
			return
		}

		resp, err := r.doRequest(ctx, body, true)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			bodySnippet := normalizeLogText(string(respBody), 320)
			log.Error("responses stream request failed: model=%s status=%d body=%s", r.modelName, resp.StatusCode, bodySnippet)
			yield(nil, fmt.Errorf("Responses API 流式错误 (HTTP %d): %s", resp.StatusCode, bodySnippet))
			return
		}

		r.processResponsesStream(resp.Body, yield)
	}
}

// processResponsesStream 处理 Responses API 的 SSE 流
func (r *ResponsesModel) processResponsesStream(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	// 默认 Scanner token 上限 64K，部分网关会返回较长 data 行，这里放宽上限。
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	// 聚合状态
	aggregatedContent := &genai.Content{Role: "model", Parts: []*genai.Part{}}
	var textContent string
	toolCallsMap := make(map[string]*responsesToolCallBuilder)
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	var currentEventType string
	var dataLines []string

	flushEvent := func() {
		if len(dataLines) == 0 {
			currentEventType = ""
			return
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" || data == "[DONE]" {
			currentEventType = ""
			return
		}
		r.handleSSEEvent(currentEventType, data, &textContent, &usageMetadata, toolCallsMap, yield)
		currentEventType = ""
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			flushEvent()
			continue
		}
		// SSE 注释/心跳行
		if strings.HasPrefix(line, ":") {
			continue
		}

		// SSE 标准 event/data 行
		if eventType, ok := strings.CutPrefix(line, "event:"); ok {
			currentEventType = strings.TrimSpace(eventType)
			continue
		}
		if data, ok := strings.CutPrefix(line, "data:"); ok {
			dataLines = append(dataLines, strings.TrimLeft(data, " "))
			continue
		}

		// 兼容部分网关直接输出 JSON 行（无 event:/data: 前缀）
		dataLines = append(dataLines, line)
	}
	flushEvent()
	if err := scanner.Err(); err != nil {
		log.Error("responses stream scanner error: model=%s err=%s", r.modelName, normalizeLogText(err.Error(), 320))
		yield(nil, fmt.Errorf("responses stream scanner error: %w", err))
		return
	}

	// 组装最终聚合响应
	if textContent != "" {
		aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{Text: textContent})
	}
	i := 0
	for _, builder := range toolCallsMap {
		callID := ensureFunctionCallID(builder.callID, builder.name, i)
		aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   callID,
				Name: builder.name,
				Args: parseJSONArgs(builder.args),
			},
		})
		i++
	}

	finalResp := &model.LLMResponse{
		Content:       aggregatedContent,
		UsageMetadata: usageMetadata,
		FinishReason:  genai.FinishReasonStop,
		Partial:       false,
		TurnComplete:  true,
	}
	yield(finalResp, nil)
}

func (r *ResponsesModel) handleSSEEvent(
	eventType string,
	data string,
	textContent *string,
	usageMetadata **genai.GenerateContentResponseUsageMetadata,
	toolCallsMap map[string]*responsesToolCallBuilder,
	yield func(*model.LLMResponse, error) bool,
) {
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		eventType = inferResponsesEventType(data)
	}

	switch eventType {
	case "response.output_text.delta":
		r.handleTextDelta(data, textContent, yield)
	case "response.output_text.done":
		r.handleTextDone(data, textContent, yield)
	case "response.refusal.delta":
		r.handleRefusalDelta(data, textContent, yield)
	case "response.refusal.done":
		r.handleRefusalDone(data, textContent, yield)
	case "response.function_call_arguments.delta":
		r.handleFuncArgsDelta(data, toolCallsMap)
	case "response.output_item.added":
		r.handleOutputItemAdded(data, toolCallsMap)
	case "response.output_item.done":
		r.handleOutputItemDone(data, toolCallsMap)
	case "response.completed":
		r.handleCompleted(data, usageMetadata, textContent, toolCallsMap)
	default:
		// 兜底：已知网关会缺失 event 字段但携带 completed 结构。
		if strings.Contains(data, `"output"`) && strings.Contains(data, `"usage"`) {
			r.handleCompleted(data, usageMetadata, textContent, toolCallsMap)
		}
	}
}

func inferResponsesEventType(data string) string {
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return ""
	}
	return strings.TrimSpace(event.Type)
}

// responsesToolCallBuilder 用于聚合流式工具调用
type responsesToolCallBuilder struct {
	itemID string
	callID string
	name   string
	args   string
}

// handleTextDelta 处理文本增量事件
func (r *ResponsesModel) handleTextDelta(data string, textContent *string, yield func(*model.LLMResponse, error) bool) {
	var delta ResponsesTextDelta
	if json.Unmarshal([]byte(data), &delta) == nil && strings.TrimSpace(delta.Delta) != "" {
		*textContent += delta.Delta
		part := &genai.Part{Text: delta.Delta}
		llmResp := &model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
			Partial:      true,
			TurnComplete: false,
		}
		yield(llmResp, nil)
		return
	}

	// 兼容 response.output_text.done 等结构
	var alt struct {
		Text  string `json:"text"`
		Delta string `json:"delta"`
	}
	if json.Unmarshal([]byte(data), &alt) != nil {
		return
	}
	text := strings.TrimSpace(alt.Delta)
	if text == "" {
		text = strings.TrimSpace(alt.Text)
	}
	if text == "" {
		return
	}
	*textContent += text
	part := &genai.Part{Text: text}
	llmResp := &model.LLMResponse{
		Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
		Partial:      true,
		TurnComplete: false,
	}
	yield(llmResp, nil)
}

func (r *ResponsesModel) handleRefusalDelta(data string, textContent *string, yield func(*model.LLMResponse, error) bool) {
	var refusal struct {
		Refusal string `json:"refusal"`
		Delta   string `json:"delta"`
		Text    string `json:"text"`
	}
	if json.Unmarshal([]byte(data), &refusal) != nil {
		return
	}
	msg := strings.TrimSpace(refusal.Refusal)
	if msg == "" {
		msg = strings.TrimSpace(refusal.Delta)
	}
	if msg == "" {
		msg = strings.TrimSpace(refusal.Text)
	}
	if msg == "" {
		return
	}
	text := "模型拒答：" + msg
	*textContent += text
	llmResp := &model.LLMResponse{
		Content:      &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}},
		Partial:      true,
		TurnComplete: false,
	}
	yield(llmResp, nil)
}

func (r *ResponsesModel) handleTextDone(data string, textContent *string, yield func(*model.LLMResponse, error) bool) {
	text := extractOutputTextFromEvent(data)
	if text == "" {
		return
	}
	appendTextAvoidDuplicate(text, textContent, yield)
}

func (r *ResponsesModel) handleRefusalDone(data string, textContent *string, yield func(*model.LLMResponse, error) bool) {
	text := extractRefusalTextFromEvent(data)
	if text == "" {
		return
	}
	appendTextAvoidDuplicate("模型拒答："+text, textContent, yield)
}

func extractOutputTextFromEvent(data string) string {
	var delta ResponsesTextDelta
	if json.Unmarshal([]byte(data), &delta) == nil && strings.TrimSpace(delta.Delta) != "" {
		return strings.TrimSpace(delta.Delta)
	}
	var alt struct {
		Text  string `json:"text"`
		Delta string `json:"delta"`
	}
	if json.Unmarshal([]byte(data), &alt) != nil {
		return ""
	}
	if strings.TrimSpace(alt.Delta) != "" {
		return strings.TrimSpace(alt.Delta)
	}
	return strings.TrimSpace(alt.Text)
}

func extractRefusalTextFromEvent(data string) string {
	var refusal struct {
		Refusal string `json:"refusal"`
		Delta   string `json:"delta"`
		Text    string `json:"text"`
	}
	if json.Unmarshal([]byte(data), &refusal) != nil {
		return ""
	}
	if strings.TrimSpace(refusal.Refusal) != "" {
		return strings.TrimSpace(refusal.Refusal)
	}
	if strings.TrimSpace(refusal.Delta) != "" {
		return strings.TrimSpace(refusal.Delta)
	}
	return strings.TrimSpace(refusal.Text)
}

func appendTextAvoidDuplicate(text string, textContent *string, yield func(*model.LLMResponse, error) bool) {
	if strings.TrimSpace(text) == "" || textContent == nil {
		return
	}
	existing := *textContent
	switch {
	case existing == "":
		*textContent = text
		yield(&model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}},
			Partial:      true,
			TurnComplete: false,
		}, nil)
	case strings.HasPrefix(text, existing):
		// 常见网关行为：done 事件携带“完整文本”，而 delta 已经累积了前半段。
		// 仅补齐缺失尾部，避免整段重复一遍。
		if len(text) > len(existing) {
			rest := text[len(existing):]
			*textContent += rest
			yield(&model.LLMResponse{
				Content:      &genai.Content{Role: "model", Parts: []*genai.Part{{Text: rest}}},
				Partial:      true,
				TurnComplete: false,
			}, nil)
		}
	case strings.HasPrefix(existing, text):
		// 已有内容包含该段，忽略。
		return
	default:
		// 非前后缀关系，按增量补充，避免丢信息。
		*textContent += text
		yield(&model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{{Text: text}}},
			Partial:      true,
			TurnComplete: false,
		}, nil)
	}
}

// handleFuncArgsDelta 处理函数调用参数增量事件
func (r *ResponsesModel) handleFuncArgsDelta(data string, toolCallsMap map[string]*responsesToolCallBuilder) {
	var delta ResponsesFuncCallArgsDelta
	if json.Unmarshal([]byte(data), &delta) != nil {
		return
	}
	if delta.ItemID == "" {
		return
	}
	builder := ensureToolCallBuilder(toolCallsMap, delta.ItemID)
	builder.args += delta.Delta
}

// handleOutputItemAdded 处理输出项添加事件
func (r *ResponsesModel) handleOutputItemAdded(data string, toolCallsMap map[string]*responsesToolCallBuilder) {
	var added ResponsesOutputItemAdded
	if json.Unmarshal([]byte(data), &added) != nil {
		return
	}
	if added.Item.Type == "function_call" && added.Item.ID != "" {
		builder := ensureToolCallBuilder(toolCallsMap, added.Item.ID)
		if added.Item.CallID != "" {
			builder.callID = added.Item.CallID
		}
		if added.Item.Name != "" {
			builder.name = added.Item.Name
		}
		if added.Item.Arguments != "" {
			builder.args = added.Item.Arguments
		}
	}
}

// handleOutputItemDone 处理输出项完成事件
func (r *ResponsesModel) handleOutputItemDone(data string, toolCallsMap map[string]*responsesToolCallBuilder) {
	var done ResponsesOutputItemDone
	if json.Unmarshal([]byte(data), &done) != nil {
		return
	}
	if done.Item.Type == "function_call" && done.Item.ID != "" {
		builder := ensureToolCallBuilder(toolCallsMap, done.Item.ID)
		if done.Item.CallID != "" {
			builder.callID = done.Item.CallID
		}
		if done.Item.Name != "" {
			builder.name = done.Item.Name
		}
		if done.Item.Arguments != "" {
			builder.args = done.Item.Arguments
		}
	}
}

func ensureToolCallBuilder(toolCallsMap map[string]*responsesToolCallBuilder, itemID string) *responsesToolCallBuilder {
	if builder, exists := toolCallsMap[itemID]; exists {
		return builder
	}
	builder := &responsesToolCallBuilder{itemID: itemID}
	toolCallsMap[itemID] = builder
	return builder
}

// handleCompleted 处理响应完成事件
func (r *ResponsesModel) handleCompleted(
	data string,
	usageMetadata **genai.GenerateContentResponseUsageMetadata,
	textContent *string,
	toolCallsMap map[string]*responsesToolCallBuilder,
) {
	var completed ResponsesCompleted
	if json.Unmarshal([]byte(data), &completed) != nil || (completed.Type == "" && completed.Response.ID == "") {
		// 兼容部分网关直接返回 response 对象（无外层 type/response）
		var direct CreateResponseResponse
		if json.Unmarshal([]byte(data), &direct) != nil {
			return
		}
		completed = ResponsesCompleted{
			Type:     "response.completed",
			Response: direct,
		}
	}
	if completed.Response.Usage != nil {
		*usageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(completed.Response.Usage.InputTokens),
			CandidatesTokenCount: int32(completed.Response.Usage.OutputTokens),
			TotalTokenCount:      int32(completed.Response.Usage.TotalTokens),
		}
	}

	// 部分网关不会发送 response.output_text.delta，而是仅在 completed 中返回最终文本。
	// 因此仅在当前未聚合到文本时，回退读取 completed.response.output / output_text。
	if textContent != nil && strings.TrimSpace(*textContent) == "" {
		if strings.TrimSpace(completed.Response.OutputText) != "" {
			*textContent = completed.Response.OutputText
		} else {
			var sb strings.Builder
			for _, item := range completed.Response.Output {
				if item.Type != "message" {
					continue
				}
				for _, part := range item.Content {
					if (part.Type == "output_text" || part.Type == "text") && part.Text != "" {
						sb.WriteString(part.Text)
					}
					if part.Type == "refusal" {
						refusalText := strings.TrimSpace(part.Refusal)
						if refusalText == "" {
							refusalText = strings.TrimSpace(part.Text)
						}
						if refusalText != "" {
							sb.WriteString("模型拒答：")
							sb.WriteString(refusalText)
						}
					}
				}
			}
			*textContent = sb.String()
		}
		if strings.TrimSpace(*textContent) != "" {
			log.Debug("responses completed fallback used: model=%s textLen=%d", r.modelName, len(*textContent))
		} else {
			log.Warn("responses completed with empty text: model=%s outputItems=%d", r.modelName, len(completed.Response.Output))
		}
	}

	// 同理，函数调用也可能仅在 completed 中出现。
	if toolCallsMap != nil {
		for _, item := range completed.Response.Output {
			if item.Type != "function_call" || item.ID == "" {
				continue
			}
			if _, exists := toolCallsMap[item.ID]; exists {
				continue
			}
			toolCallsMap[item.ID] = &responsesToolCallBuilder{
				itemID: item.ID,
				callID: item.CallID,
				name:   item.Name,
				args:   item.Arguments,
			}
		}
	}
}
