package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
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

// processStream 处理流式响应
func (o *OpenAIModel) processStream(stream *openai.ChatCompletionStream, yield func(*model.LLMResponse, error) bool) {
	aggregatedContent := &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{},
	}
	var finishReason genai.FinishReason
	var usageMetadata *genai.GenerateContentResponseUsageMetadata
	toolCalls := newChatStreamToolCallAggregator()
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
		for pos, toolCall := range choice.Delta.ToolCalls {
			toolCalls.AddDelta(pos, toolCall)
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
	for _, builder := range toolCalls.OrderedBuilders() {
		if builder == nil {
			continue
		}
		part := &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   builder.id,
				Name: builder.name,
				Args: parseJSONArgs(builder.args),
			},
		}
		aggregatedContent.Parts = append(aggregatedContent.Parts, part)
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

// chatStreamToolCallAggregator 聚合兼容 OpenAI 的流式工具调用。
// 一些兼容接口不会稳定返回 index，这里优先按 index / id 归并，
// 都缺失时再按当前位置兜底，并在检测到新 JSON 对象时自动切分。
type chatStreamToolCallAggregator struct {
	builders      map[string]*toolCallBuilder
	order         []string
	indexKeys     map[int]string
	idKeys        map[string]string
	fallbackKeys  map[int]string
	nextSynthetic int
}

func newChatStreamToolCallAggregator() *chatStreamToolCallAggregator {
	return &chatStreamToolCallAggregator{
		builders:     make(map[string]*toolCallBuilder),
		indexKeys:    make(map[int]string),
		idKeys:       make(map[string]string),
		fallbackKeys: make(map[int]string),
	}
}

func (a *chatStreamToolCallAggregator) AddDelta(pos int, toolCall openai.ToolCall) {
	key := a.lookupKey(pos, toolCall)
	if builder := a.builders[key]; shouldRotateToolCallBuilder(builder, toolCall) {
		key = a.newSyntheticKey()
	}

	a.bindAliases(pos, toolCall, key)
	builder := a.ensureBuilder(key)
	if toolCall.ID != "" {
		builder.id = toolCall.ID
	}
	if toolCall.Function.Name != "" {
		builder.name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		builder.args += toolCall.Function.Arguments
	}
}

func (a *chatStreamToolCallAggregator) OrderedBuilders() []*toolCallBuilder {
	builders := make([]*toolCallBuilder, 0, len(a.order))
	for _, key := range a.order {
		builders = append(builders, a.builders[key])
	}
	return builders
}

func (a *chatStreamToolCallAggregator) lookupKey(pos int, toolCall openai.ToolCall) string {
	if toolCall.Index != nil {
		if key, ok := a.indexKeys[*toolCall.Index]; ok {
			return key
		}
	}
	if toolCall.ID != "" {
		if key, ok := a.idKeys[toolCall.ID]; ok {
			return key
		}
	}
	if key, ok := a.fallbackKeys[pos]; ok {
		return key
	}
	if toolCall.Index != nil {
		return fmt.Sprintf("idx:%d", *toolCall.Index)
	}
	if toolCall.ID != "" {
		return fmt.Sprintf("id:%s", toolCall.ID)
	}
	return a.newSyntheticKey()
}

func (a *chatStreamToolCallAggregator) bindAliases(pos int, toolCall openai.ToolCall, key string) {
	if toolCall.Index != nil {
		a.indexKeys[*toolCall.Index] = key
	}
	if toolCall.ID != "" {
		a.idKeys[toolCall.ID] = key
	}
	if toolCall.Index == nil {
		a.fallbackKeys[pos] = key
	}
}

func (a *chatStreamToolCallAggregator) ensureBuilder(key string) *toolCallBuilder {
	if builder, ok := a.builders[key]; ok {
		return builder
	}
	builder := &toolCallBuilder{}
	a.builders[key] = builder
	a.order = append(a.order, key)
	return builder
}

func (a *chatStreamToolCallAggregator) newSyntheticKey() string {
	key := fmt.Sprintf("anon:%d", a.nextSynthetic)
	a.nextSynthetic++
	return key
}

func shouldRotateToolCallBuilder(builder *toolCallBuilder, toolCall openai.ToolCall) bool {
	if builder == nil {
		return false
	}
	if toolCall.ID != "" && builder.id != "" && toolCall.ID != builder.id {
		return true
	}
	if toolCall.Function.Name != "" && builder.name != "" && toolCall.Function.Name != builder.name {
		return true
	}

	if !isCompleteJSONObject(builder.args) {
		return false
	}

	if toolCall.Function.Arguments != "" {
		return true
	}
	if toolCall.ID != "" && builder.id == "" {
		return true
	}
	return false
}

func isCompleteJSONObject(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	return json.Valid([]byte(trimmed))
}
