package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/run-bigpig/jcp/internal/logger"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var _ model.LLM = &OpenAIModel{}

var (
	ErrNoChoicesInResponse = errors.New("no choices in OpenAI response")
)

var log = logger.New("OpenAI")

// OpenAIModel 实现 model.LLM 接口，支持 thinking 模型
type OpenAIModel struct {
	Client     *openai.Client
	ModelName  string
	BaseURL    string
	APIKey     string
	HTTPClient openai.HTTPDoer
}

// NewOpenAIModel 创建 OpenAI 模型
func NewOpenAIModel(modelName string, cfg openai.ClientConfig, apiKey string, baseURL string, httpClient openai.HTTPDoer) *OpenAIModel {
	client := openai.NewClientWithConfig(cfg)
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &OpenAIModel{
		Client:     client,
		ModelName:  modelName,
		BaseURL:    baseURL,
		APIKey:     apiKey,
		HTTPClient: httpClient,
	}
}

// Name 返回模型名称
func (o *OpenAIModel) Name() string {
	return o.ModelName
}

// GenerateContent 实现 model.LLM 接口
func (o *OpenAIModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		if isReasoningModel(o.ModelName) {
			return o.generateRaw(ctx, req)
		}
		return o.generateStream(ctx, req)
	}
	if isReasoningModel(o.ModelName) {
		return o.generateRaw(ctx, req)
	}
	return o.generate(ctx, req)
}

// generate 非流式生成
func (o *OpenAIModel) generate(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName)
		if err != nil {
			yield(nil, err)
			return
		}
		if isReasoningModel(o.ModelName) {
			log.Debug("reasoning model request: model=%s temp=%.2f top_p=%.2f n=%d presence=%.2f frequency=%.2f",
				o.ModelName, openaiReq.Temperature, openaiReq.TopP, openaiReq.N, openaiReq.PresencePenalty, openaiReq.FrequencyPenalty)
		}

		resp, err := o.Client.CreateChatCompletion(ctx, openaiReq)
		if err != nil {
			yield(nil, err)
			return
		}

		llmResp, err := convertChatCompletionResponse(&resp)
		if err != nil {
			yield(nil, err)
			return
		}

		yield(llmResp, nil)
	}
}

func (o *OpenAIModel) generateRaw(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName)
		if err != nil {
			yield(nil, err)
			return
		}
		openaiReq.Stream = false
		log.Debug("reasoning model raw request: model=%s temp=%.2f top_p=%.2f n=%d presence=%.2f frequency=%.2f",
			o.ModelName, openaiReq.Temperature, openaiReq.TopP, openaiReq.N, openaiReq.PresencePenalty, openaiReq.FrequencyPenalty)

		body, err := json.Marshal(openaiReq)
		if err != nil {
			yield(nil, err)
			return
		}

		url := strings.TrimRight(o.BaseURL, "/") + "/chat/completions"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			yield(nil, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "application/json")
		if o.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
		}

		resp, err := o.HTTPClient.Do(httpReq)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
			bodySnippet := normalizeLogText(string(respBody), 320)
			log.Error("openai raw request failed: model=%s status=%d url=%s body=%s", o.ModelName, resp.StatusCode, url, bodySnippet)
			yield(nil, fmt.Errorf("openai raw request failed (status=%d): %s", resp.StatusCode, bodySnippet))
			return
		}

		var parsed openai.ChatCompletionResponse
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			yield(nil, err)
			return
		}

		llmResp, err := convertChatCompletionResponse(&parsed)
		if err != nil {
			yield(nil, err)
			return
		}
		yield(llmResp, nil)
	}
}

// generateStream 流式生成
func (o *OpenAIModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName)
		if err != nil {
			yield(nil, err)
			return
		}
		openaiReq.Stream = true
		if isReasoningModel(o.ModelName) {
			log.Debug("reasoning model stream: model=%s temp=%.2f top_p=%.2f n=%d presence=%.2f frequency=%.2f",
				o.ModelName, openaiReq.Temperature, openaiReq.TopP, openaiReq.N, openaiReq.PresencePenalty, openaiReq.FrequencyPenalty)
		}

		stream, err := o.Client.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			log.Error("openai stream request failed: model=%s err=%s", o.ModelName, normalizeLogText(err.Error(), 320))
			yield(nil, err)
			return
		}
		defer stream.Close()

		o.processStream(stream, yield)
	}
}

// processStream 处理流式响应
func (o *OpenAIModel) processStream(stream *openai.ChatCompletionStream, yield func(*model.LLMResponse, error) bool) {
	aggregatedContent := &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{},
	}
	var finishReason genai.FinishReason
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	toolCallsMap := make(map[int]*toolCallBuilder)
	var textContent string
	var reasoningContent string

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Error("openai stream recv failed: model=%s err=%s", o.ModelName, normalizeLogText(err.Error(), 320))
			yield(nil, fmt.Errorf("openai stream recv failed: %w", err))
			return
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 处理 reasoning_content (thinking 模型)
		if choice.Delta.ReasoningContent != "" {
			reasoningContent += choice.Delta.ReasoningContent
			// 发送 thinking 部分
			part := &genai.Part{Text: choice.Delta.ReasoningContent, Thought: true}
			llmResp := &model.LLMResponse{
				Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
				Partial:      true,
				TurnComplete: false,
			}
			if !yield(llmResp, nil) {
				return
			}
		}

		// 处理普通文本内容
		if choice.Delta.Content != "" {
			textContent += choice.Delta.Content
			part := &genai.Part{Text: choice.Delta.Content}
			llmResp := &model.LLMResponse{
				Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
				Partial:      true,
				TurnComplete: false,
			}
			if !yield(llmResp, nil) {
				return
			}
		}

		// 处理工具调用
		for _, toolCall := range choice.Delta.ToolCalls {
			idx := 0
			if toolCall.Index != nil {
				idx = *toolCall.Index
			}

			if _, exists := toolCallsMap[idx]; !exists {
				toolCallsMap[idx] = &toolCallBuilder{}
			}

			builder := toolCallsMap[idx]
			if toolCall.ID != "" {
				builder.id = toolCall.ID
			}
			if toolCall.Function.Name != "" {
				builder.name = toolCall.Function.Name
			}
			builder.args += toolCall.Function.Arguments
		}

		// 处理结束原因
		if choice.FinishReason != "" {
			finishReason = convertFinishReason(string(choice.FinishReason))
		}

		// 处理 usage
		if chunk.Usage != nil {
			usageMetadata = &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     int32(chunk.Usage.PromptTokens),
				CandidatesTokenCount: int32(chunk.Usage.CompletionTokens),
				TotalTokenCount:      int32(chunk.Usage.TotalTokens),
			}
		}
	}

	// 添加聚合的文本内容
	if textContent != "" {
		aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{Text: textContent})
	}

	// 添加 reasoning content 作为 thought part
	if reasoningContent != "" {
		aggregatedContent.Parts = append([]*genai.Part{{Text: reasoningContent, Thought: true}}, aggregatedContent.Parts...)
	}

	// 添加工具调用
	if len(toolCallsMap) > 0 {
		indices := sortedKeys(toolCallsMap)
		for i, idx := range indices {
			builder := toolCallsMap[idx]
			callID := ensureFunctionCallID(builder.id, builder.name, i)
			part := &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   callID,
					Name: builder.name,
					Args: parseJSONArgs(builder.args),
				},
			}
			aggregatedContent.Parts = append(aggregatedContent.Parts, part)
		}
	}

	// 发送最终响应
	finalResp := &model.LLMResponse{
		Content:       aggregatedContent,
		UsageMetadata: usageMetadata,
		FinishReason:  finishReason,
		Partial:       false,
		TurnComplete:  true,
	}
	yield(finalResp, nil)
}

// toolCallBuilder 用于聚合流式工具调用
type toolCallBuilder struct {
	id   string
	name string
	args string
}

// sortedKeys 返回排序后的 map keys
func sortedKeys(m map[int]*toolCallBuilder) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单冒泡排序
	for i := 0; i < len(keys)-1; i++ {
		for j := 0; j < len(keys)-i-1; j++ {
			if keys[j] > keys[j+1] {
				keys[j], keys[j+1] = keys[j+1], keys[j]
			}
		}
	}
	return keys
}
