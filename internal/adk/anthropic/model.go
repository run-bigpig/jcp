package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/run-bigpig/jcp/internal/logger"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

var modelLog = logger.New("anthropic:model")

// 确保实现 model.LLM 接口
var _ model.LLM = &AnthropicModel{}

// AnthropicModel Anthropic Messages API 模型
type AnthropicModel struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	modelName  string
}

func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return baseURL
}

// NewAnthropicModel 创建 Anthropic 模型
func NewAnthropicModel(modelName, apiKey, baseURL string, httpClient *http.Client) *AnthropicModel {
	return &AnthropicModel{
		httpClient: httpClient,
		baseURL:    normalizeBaseURL(baseURL),
		apiKey:     apiKey,
		modelName:  modelName,
	}
}

// Name 返回模型名称
func (m *AnthropicModel) Name() string {
	return m.modelName
}

// GenerateContent 实现 model.LLM 接口
func (m *AnthropicModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	if stream {
		return m.generateStream(ctx, req)
	}
	return m.generate(ctx, req)
}

// doRequest 发送 HTTP 请求到 Anthropic API
func (m *AnthropicModel) doRequest(ctx context.Context, ar *MessagesRequest) (*http.Response, error) {
	jsonBody, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint, err := url.JoinPath(m.baseURL, "v1", "messages")
	if err != nil {
		return nil, fmt.Errorf("build endpoint: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", m.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	return m.httpClient.Do(httpReq)
}

// generate 非流式生成
func (m *AnthropicModel) generate(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		ar, err := toAnthropicRequest(req, m.modelName)
		if err != nil {
			yield(nil, err)
			return
		}
		ar.Stream = false

		resp, err := m.doRequest(ctx, ar)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		if err != nil {
			yield(nil, fmt.Errorf("read response: %w", err))
			return
		}

		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
			return
		}

		var msgResp MessagesResponse
		if err := json.Unmarshal(body, &msgResp); err != nil {
			yield(nil, fmt.Errorf("unmarshal response: %w", err))
			return
		}

		llmResp, err := convertAnthropicResponse(&msgResp)
		if err != nil {
			yield(nil, err)
			return
		}

		yield(llmResp, nil)
	}
}

// generateStream 流式生成
func (m *AnthropicModel) generateStream(ctx context.Context, req *model.LLMRequest) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		ar, err := toAnthropicRequest(req, m.modelName)
		if err != nil {
			yield(nil, err)
			return
		}
		ar.Stream = true

		resp, err := m.doRequest(ctx, ar)
		if err != nil {
			yield(nil, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			yield(nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body)))
			return
		}

		m.processStream(resp.Body, yield)
	}
}

// blockState 跟踪流式内容块状态
type blockState struct {
	blockType string // text / tool_use / thinking
	toolID    string
	toolName  string
	text      string
	thinking  string
	toolArgs  string
}

// processStream 处理 SSE 事件流
func (m *AnthropicModel) processStream(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 1MB buffer

	aggregated := &genai.Content{
		Role:  "model",
		Parts: []*genai.Part{},
	}
	var stopReason string
	var usage *Usage
	blocks := make(map[int]*blockState)
	var eventType string

	for scanner.Scan() {
		line := scanner.Text()

		// SSE 事件类型行
		if ev, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = ev
			continue
		}

		// SSE 数据行
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}

		if err := m.handleSSEEvent(eventType, []byte(data), blocks, &stopReason, &usage, yield); err != nil {
			if errors.Is(err, errStopIteration) {
				return
			}
			yield(nil, err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			yield(nil, fmt.Errorf("SSE 读取错误: %w", err))
		}
		return
	}

	// 发送最终聚合响应
	m.emitFinalResponse(aggregated, blocks, stopReason, usage, yield)
}

var errStopIteration = errors.New("stop iteration")

// handleSSEEvent 处理单个 SSE 事件
func (m *AnthropicModel) handleSSEEvent(
	eventType string, data []byte,
	blocks map[int]*blockState,
	stopReason *string, usage **Usage,
	yield func(*model.LLMResponse, error) bool,
) error {
	switch eventType {
	case "message_start":
		var ev SSEMessageStart
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil // 忽略解析错误
		}
		u := ev.Message.Usage
		*usage = &u

	case "content_block_start":
		var ev SSEContentBlockStart
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil
		}
		bs := &blockState{blockType: ev.ContentBlock.Type}
		if ev.ContentBlock.Type == "tool_use" {
			bs.toolID = ev.ContentBlock.ID
			bs.toolName = ev.ContentBlock.Name
		}
		blocks[ev.Index] = bs

	case "content_block_delta":
		return m.handleDelta(data, blocks, yield)

	case "content_block_stop":
		// 不需要特殊处理，最终聚合时处理

	case "message_delta":
		var ev SSEMessageDelta
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil
		}
		*stopReason = ev.Delta.StopReason
		if ev.Usage != nil {
			*usage = ev.Usage
		}

	case "message_stop":
		// 流结束，processStream 循环退出后会发送最终响应

	case "error":
		var ev SSEError
		if err := json.Unmarshal(data, &ev); err != nil {
			return fmt.Errorf("SSE error: %s", string(data))
		}
		return fmt.Errorf("Anthropic API error: %s - %s", ev.Error.Type, ev.Error.Message)

	case "ping":
		// 忽略
	}

	return nil
}

// handleDelta 处理 content_block_delta 事件
func (m *AnthropicModel) handleDelta(
	data []byte, blocks map[int]*blockState,
	yield func(*model.LLMResponse, error) bool,
) error {
	var ev SSEContentBlockDelta
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil
	}

	bs, ok := blocks[ev.Index]
	if !ok {
		return nil
	}

	switch ev.Delta.Type {
	case "text_delta":
		bs.text += ev.Delta.Text
		part := &genai.Part{Text: ev.Delta.Text}
		resp := &model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
			Partial:      true,
			TurnComplete: false,
		}
		if !yield(resp, nil) {
			return errStopIteration
		}

	case "thinking_delta":
		bs.thinking += ev.Delta.Thinking
		part := &genai.Part{Text: ev.Delta.Thinking, Thought: true}
		resp := &model.LLMResponse{
			Content:      &genai.Content{Role: "model", Parts: []*genai.Part{part}},
			Partial:      true,
			TurnComplete: false,
		}
		if !yield(resp, nil) {
			return errStopIteration
		}

	case "input_json_delta":
		bs.toolArgs += ev.Delta.PartialJSON
	}

	return nil
}

// emitFinalResponse 聚合所有块并发送最终响应
func (m *AnthropicModel) emitFinalResponse(
	aggregated *genai.Content,
	blocks map[int]*blockState,
	stopReason string, usage *Usage,
	yield func(*model.LLMResponse, error) bool,
) {
	// 按 index 顺序聚合所有块，避免 map 非连续索引导致内容丢失。
	indices := make([]int, 0, len(blocks))
	for idx := range blocks {
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	for _, idx := range indices {
		bs := blocks[idx]

		switch bs.blockType {
		case "thinking":
			if bs.thinking != "" {
				aggregated.Parts = append(aggregated.Parts, &genai.Part{
					Text: bs.thinking, Thought: true,
				})
			}
		case "text":
			if bs.text != "" {
				aggregated.Parts = append(aggregated.Parts, &genai.Part{
					Text: bs.text,
				})
			}
		case "tool_use":
			args := make(map[string]any)
			if bs.toolArgs != "" {
				if err := json.Unmarshal([]byte(bs.toolArgs), &args); err != nil {
					modelLog.Warn("解析 tool_use args 失败: %v", err)
				}
			}
			aggregated.Parts = append(aggregated.Parts, &genai.Part{
				FunctionCall: &genai.FunctionCall{
					ID:   bs.toolID,
					Name: bs.toolName,
					Args: args,
				},
			})
		}
	}

	finalResp := &model.LLMResponse{
		Content:       aggregated,
		UsageMetadata: convertUsage(usage),
		FinishReason:  convertStopReason(stopReason),
		Partial:       false,
		TurnComplete:  true,
	}
	yield(finalResp, nil)
}
