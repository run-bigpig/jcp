package adk

import (
	"context"
	"iter"
	"strings"

	"github.com/run-bigpig/jcp/internal/logger"
	"google.golang.org/adk/model"
)

var fallbackLog = logger.New("LLMFallback")

// fallbackModel 在主模型不可用（报错/空响应）时，自动回退到备用模型。
type fallbackModel struct {
	name      string
	primary   model.LLM
	secondary model.LLM
}

func newFallbackModel(primary model.LLM, secondary model.LLM, name string) model.LLM {
	if primary == nil {
		return secondary
	}
	if secondary == nil {
		return primary
	}
	if strings.TrimSpace(name) == "" {
		name = primary.Name()
	}
	return &fallbackModel{
		name:      name,
		primary:   primary,
		secondary: secondary,
	}
}

func (m *fallbackModel) Name() string {
	return m.name
}

func (m *fallbackModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		var buffered []*model.LLMResponse
		var primaryErr error
		primaryUsable := false

		for resp, err := range m.primary.GenerateContent(ctx, req, stream) {
			if err != nil {
				primaryErr = err
				break
			}
			if resp != nil {
				buffered = append(buffered, resp)
				if llmResponseUsable(resp) {
					primaryUsable = true
				}
			}
		}

		if primaryErr == nil && primaryUsable {
			for _, resp := range buffered {
				if !yield(resp, nil) {
					return
				}
			}
			return
		}

		if primaryErr != nil {
			fallbackLog.Warn("primary model failed, fallback to secondary: err=%v", primaryErr)
		} else {
			fallbackLog.Warn("primary model produced no usable output, fallback to secondary")
		}

		for resp, err := range m.secondary.GenerateContent(ctx, req, stream) {
			if !yield(resp, err) {
				return
			}
		}
	}
}

func llmResponseUsable(resp *model.LLMResponse) bool {
	if resp == nil || resp.Content == nil {
		return false
	}

	for _, part := range resp.Content.Parts {
		if part == nil {
			continue
		}
		if part.FunctionCall != nil || part.FunctionResponse != nil {
			return true
		}
		if !part.Thought && strings.TrimSpace(part.Text) != "" {
			return true
		}
	}
	return false
}
