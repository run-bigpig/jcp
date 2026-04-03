package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"
	"strings"

	"github.com/sashabaranov/go-openai"
	"google.golang.org/adk/model"
	"google.golang.org/genai"

	"github.com/run-bigpig/jcp/internal/logger"
)

var modelLog = logger.New("openai:model")

var _ model.LLM = &OpenAIModel{}

var (
	ErrNoChoicesInResponse = errors.New("no choices in OpenAI response")
)

// OpenAIModel 实现 model.LLM 接口，支持 thinking 模型
type OpenAIModel struct {
	Client         *openai.Client
	ModelName      string
	TokenParamMode string
	NoSystemRole   bool // 不支持 system role 时需要降级处理
}

// NewOpenAIModel 创建 OpenAI 模型
func NewOpenAIModel(modelName string, cfg openai.ClientConfig, noSystemRole bool, tokenParamMode string) *OpenAIModel {
	client := openai.NewClientWithConfig(cfg)
	return &OpenAIModel{
		Client:         client,
		ModelName:      modelName,
		TokenParamMode: tokenParamMode,
		NoSystemRole:   noSystemRole,
	}
}

// Name 返回模型名称
func (o *OpenAIModel) Name() string {
	return o.ModelName
}

// GenerateContent 实现 model.LLM 接口
func (o *OpenAIModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return o.generateStream(ctx, req)
	}
	return o.generate(ctx, req)
}

// generate 非流式生成
func (o *OpenAIModel) generate(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.NoSystemRole, o.TokenParamMode)
		if err != nil {
			yield(nil, err)
			return
		}

		resp, err := o.Client.CreateChatCompletion(ctx, openaiReq)
		if err != nil {
			retryReq, ok := buildCompatRetryRequest(openaiReq, err)
			if ok {
				modelLog.Warn("模型 [%s] 首次请求参数不兼容，已自动调整后重试: %v", o.ModelName, err)
				resp, err = o.Client.CreateChatCompletion(ctx, retryReq)
			}
		}
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

// generateStream 流式生成
func (o *OpenAIModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		openaiReq, err := toOpenAIChatCompletionRequest(req, o.ModelName, o.NoSystemRole, o.TokenParamMode)
		if err != nil {
			yield(nil, err)
			return
		}
		openaiReq.Stream = true

		stream, err := o.Client.CreateChatCompletionStream(ctx, openaiReq)
		if err != nil {
			retryReq, ok := buildCompatRetryRequest(openaiReq, err)
			if ok {
				retryReq.Stream = true
				modelLog.Warn("模型 [%s] 首次流式请求参数不兼容，已自动调整后重试: %v", o.ModelName, err)
				stream, err = o.Client.CreateChatCompletionStream(ctx, retryReq)
			}
		}
		if err != nil {
			yield(nil, err)
			return
		}
		defer stream.Close()

		o.processStream(stream, yield)
	}
}

func buildCompatRetryRequest(req openai.ChatCompletionRequest, err error) (openai.ChatCompletionRequest, bool) {
	if !isCompatRetryableError(err) {
		return req, false
	}

	changed := false
	if req.MaxTokens > 0 && req.MaxCompletionTokens == 0 {
		req.MaxCompletionTokens = req.MaxTokens
		req.MaxTokens = 0
		changed = true
	}
	if req.Temperature != 0 && req.Temperature != 1 {
		req.Temperature = 1
		changed = true
	}
	if req.TopP != 0 && req.TopP != 1 {
		req.TopP = 1
		changed = true
	}
	if req.N != 0 && req.N != 1 {
		req.N = 1
		changed = true
	}
	if req.PresencePenalty != 0 {
		req.PresencePenalty = 0
		changed = true
	}
	if req.FrequencyPenalty != 0 {
		req.FrequencyPenalty = 0
		changed = true
	}

	return req, changed
}

func isCompatRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, openai.ErrReasoningModelMaxTokensDeprecated) ||
		errors.Is(err, openai.ErrReasoningModelLimitationsOther) ||
		errors.Is(err, openai.ErrO1MaxTokensDeprecated) ||
		errors.Is(err, openai.ErrO1BetaLimitationsOther) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "please use maxcompletiontokens") ||
		strings.Contains(msg, "please use max_completion_tokens") ||
		strings.Contains(msg, "temperature, top_p and n are fixed at 1")
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
	var thoughtContent string
	thinkParser := newThinkTagStreamParser()

	emitPartial := func(seg thinkSegment) bool {
		if seg.Text == "" {
			return true
		}
		if seg.Thought {
			thoughtContent += seg.Text
		} else {
			textContent += seg.Text
		}

		part := &genai.Part{Text: seg.Text, Thought: seg.Thought}
		llmResp := &model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
			Partial:      true,
			TurnComplete: false,
		}
		return yield(llmResp, nil)
	}

	var streamErr error
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				streamErr = fmt.Errorf("流式读取错误: %w", err)
				modelLog.Warn("流式读取中断: %v", err)
			}
			break
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// 官方 reasoning_content -> Thought
		if choice.Delta.ReasoningContent != "" {
			if !emitPartial(thinkSegment{
				Text:    choice.Delta.ReasoningContent,
				Thought: true,
			}) {
				return
			}
		}

		// content 中的 <think>...</think> -> Thought
		for _, seg := range thinkParser.Feed(choice.Delta.Content) {
			if !emitPartial(seg) {
				return
			}
		}

		// 处理标准工具调用
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

		if choice.FinishReason != "" {
			finishReason = convertFinishReason(string(choice.FinishReason))
		}

		if chunk.Usage != nil {
			usageMetadata = &genai.GenerateContentResponseUsageMetadata{
				PromptTokenCount:     int32(chunk.Usage.PromptTokens),
				CandidatesTokenCount: int32(chunk.Usage.CompletionTokens),
				TotalTokenCount:      int32(chunk.Usage.TotalTokens),
			}
		}
	}

	// 刷新流式标签解析器（处理标签跨 chunk 场景）
	for _, seg := range thinkParser.Flush() {
		if !emitPartial(seg) {
			return
		}
	}

	// 聚合文本并解析第三方工具调用标记
	if textContent != "" {
		vendorCalls, cleanedText := parseVendorToolCalls(textContent)
		if cleanedText != "" {
			aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{Text: cleanedText})
		}
		for i, vc := range vendorCalls {
			aggregatedContent.Parts = append(aggregatedContent.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   fmt.Sprintf("vendor_call_%d", i),
					Name: vc.Name,
					Args: vc.Args,
				},
			})
		}
	}

	if thoughtContent != "" {
		aggregatedContent.Parts = append([]*genai.Part{{Text: thoughtContent, Thought: true}}, aggregatedContent.Parts...)
	}

	// 聚合标准工具调用
	if len(toolCallsMap) > 0 {
		indices := sortedKeys(toolCallsMap)
		for _, idx := range indices {
			builder := toolCallsMap[idx]
			part := &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   builder.id,
					Name: builder.name,
					Args: parseJSONArgs(builder.args),
				},
			}
			aggregatedContent.Parts = append(aggregatedContent.Parts, part)
		}
	}

	if streamErr != nil {
		yield(nil, streamErr)
		return
	}

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
	slices.Sort(keys)
	return keys
}
