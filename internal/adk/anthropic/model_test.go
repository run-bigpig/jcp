package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

func TestToAnthropicRequest_Basic(t *testing.T) {
	temp := float32(0.7)
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "hello"}}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 1024,
			Temperature:     &temp,
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: "You are helpful."}},
			},
		},
	}

	ar, err := toAnthropicRequest(req, "claude-opus-4-6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ar.Model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", ar.Model, "claude-opus-4-6")
	}
	if ar.MaxTokens != 1024 {
		t.Errorf("max_tokens = %d, want 1024", ar.MaxTokens)
	}
	if ar.System != "You are helpful." {
		t.Errorf("system = %q, want %q", ar.System, "You are helpful.")
	}
	if ar.Temperature == nil {
		t.Error("temperature is nil")
	} else if diff := *ar.Temperature - 0.7; diff > 0.01 || diff < -0.01 {
		t.Errorf("temperature = %f, want ~0.7", *ar.Temperature)
	}
	if len(ar.Messages) != 1 || ar.Messages[0].Role != "user" {
		t.Fatalf("messages unexpected: %+v", ar.Messages)
	}
	if ar.Messages[0].Content[0].Text != "hello" {
		t.Errorf("message text = %q, want %q", ar.Messages[0].Content[0].Text, "hello")
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "trim trailing slash", in: "https://api.anthropic.com/", want: "https://api.anthropic.com"},
		{name: "strip v1", in: "https://api.anthropic.com/v1", want: "https://api.anthropic.com"},
		{name: "strip v1 and slash", in: "https://proxy.example.com/base/v1/", want: "https://proxy.example.com/base"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeBaseURL(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewAnthropicModel_NormalizesBaseURL(t *testing.T) {
	m := NewAnthropicModel("claude-sonnet-4", "key", "https://api.anthropic.com/v1/", http.DefaultClient)
	if strings.Contains(m.baseURL, "/v1") {
		t.Fatalf("expected normalized baseURL without /v1, got %q", m.baseURL)
	}
}

func TestToAnthropicMessages_ToolUseAndResult(t *testing.T) {
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "What's the weather?"}}},
		{Role: "model", Parts: []*genai.Part{
			{Text: "Let me check."},
			{FunctionCall: &genai.FunctionCall{
				ID:   "call_123",
				Name: "get_weather",
				Args: map[string]any{"city": "Tokyo"},
			}},
		}},
		{Role: "user", Parts: []*genai.Part{
			{FunctionResponse: &genai.FunctionResponse{
				ID:       "call_123",
				Response: map[string]any{"temp": "20C"},
			}},
		}},
	}

	msgs, err := toAnthropicMessages(contents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3", len(msgs))
	}

	// assistant message should have text + tool_use
	assistantBlocks := msgs[1].Content
	if len(assistantBlocks) != 2 {
		t.Fatalf("assistant has %d blocks, want 2", len(assistantBlocks))
	}
	if assistantBlocks[0].Type != "text" {
		t.Errorf("block[0].type = %q, want text", assistantBlocks[0].Type)
	}
	if assistantBlocks[1].Type != "tool_use" {
		t.Errorf("block[1].type = %q, want tool_use", assistantBlocks[1].Type)
	}
	if assistantBlocks[1].Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", assistantBlocks[1].Name)
	}

	// user message should have tool_result
	userBlocks := msgs[2].Content
	if len(userBlocks) != 1 || userBlocks[0].Type != "tool_result" {
		t.Fatalf("user block unexpected: %+v", userBlocks)
	}
	if userBlocks[0].ToolUseID != "call_123" {
		t.Errorf("tool_use_id = %q, want call_123", userBlocks[0].ToolUseID)
	}
	var toolResultContent string
	if err := json.Unmarshal(userBlocks[0].RawContent, &toolResultContent); err != nil {
		t.Fatalf("tool_result content should be string JSON, got: %s, err=%v", string(userBlocks[0].RawContent), err)
	}
	if toolResultContent != `{"temp":"20C"}` {
		t.Errorf("tool_result content = %q, want %q", toolResultContent, `{"temp":"20C"}`)
	}
}

func TestToToolResultContent_StringAndObject(t *testing.T) {
	rawText, err := toToolResultContent("ok")
	if err != nil {
		t.Fatalf("unexpected error for text: %v", err)
	}
	var text string
	if err := json.Unmarshal(rawText, &text); err != nil {
		t.Fatalf("unmarshal raw text failed: %v", err)
	}
	if text != "ok" {
		t.Errorf("text = %q, want %q", text, "ok")
	}

	rawObj, err := toToolResultContent(map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("unexpected error for object: %v", err)
	}
	var objAsText string
	if err := json.Unmarshal(rawObj, &objAsText); err != nil {
		t.Fatalf("object content should be JSON string: %v", err)
	}
	if objAsText != `{"x":1}` {
		t.Errorf("object content = %q, want %q", objAsText, `{"x":1}`)
	}
}

func TestConvertAnthropicResponse_TextAndToolUse(t *testing.T) {
	resp := &MessagesResponse{
		ID:         "msg_123",
		Role:       "assistant",
		StopReason: "tool_use",
		Content: []ContentBlock{
			{Type: "text", Text: "Let me check."},
			{
				Type:  "tool_use",
				ID:    "tu_456",
				Name:  "get_weather",
				Input: json.RawMessage(`{"city":"Tokyo"}`),
			},
		},
		Usage: Usage{InputTokens: 10, OutputTokens: 20},
	}

	llmResp, err := convertAnthropicResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !llmResp.TurnComplete {
		t.Error("expected TurnComplete=true")
	}
	parts := llmResp.Content.Parts
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(parts))
	}
	if parts[0].Text != "Let me check." {
		t.Errorf("text = %q", parts[0].Text)
	}
	if parts[1].FunctionCall == nil {
		t.Fatal("expected FunctionCall")
	}
	if parts[1].FunctionCall.Name != "get_weather" {
		t.Errorf("tool name = %q", parts[1].FunctionCall.Name)
	}
	if llmResp.UsageMetadata.TotalTokenCount != 30 {
		t.Errorf("total tokens = %d, want 30", llmResp.UsageMetadata.TotalTokenCount)
	}
}

func TestConvertStopReason(t *testing.T) {
	tests := []struct {
		reason string
		want   genai.FinishReason
	}{
		{"end_turn", genai.FinishReasonStop},
		{"max_tokens", genai.FinishReasonMaxTokens},
		{"tool_use", genai.FinishReasonStop},
		{"stop_sequence", genai.FinishReasonStop},
		{"unknown", genai.FinishReasonUnspecified},
	}
	for _, tt := range tests {
		got := convertStopReason(tt.reason)
		if got != tt.want {
			t.Errorf("convertStopReason(%q) = %v, want %v", tt.reason, got, tt.want)
		}
	}
}

func TestToAnthropicMessages_MergeConsecutiveRoles(t *testing.T) {
	// Anthropic 要求 user/assistant 交替，相同 role 应合并
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "part1"}}},
		{Role: "user", Parts: []*genai.Part{{Text: "part2"}}},
	}

	msgs, err := toAnthropicMessages(contents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1 (should merge)", len(msgs))
	}
	if len(msgs[0].Content) != 2 {
		t.Errorf("got %d blocks, want 2", len(msgs[0].Content))
	}
}

// 集成测试：需要设置环境变量 ANTHROPIC_TEST_URL 和 ANTHROPIC_TEST_KEY
func TestIntegration_NonStreaming(t *testing.T) {
	baseURL := os.Getenv("ANTHROPIC_TEST_URL")
	apiKey := os.Getenv("ANTHROPIC_TEST_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("跳过集成测试：未设置 ANTHROPIC_TEST_URL / ANTHROPIC_TEST_KEY")
	}

	m := NewAnthropicModel("claude-opus-4-6", apiKey, baseURL, http.DefaultClient)
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Reply with exactly: PONG"}}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 32,
		},
	}

	ctx := context.Background()
	for resp, err := range m.GenerateContent(ctx, req, false) {
		if err != nil {
			t.Fatalf("non-streaming error: %v", err)
		}
		if resp == nil || resp.Content == nil || len(resp.Content.Parts) == 0 {
			t.Fatal("empty response")
		}
		t.Logf("non-streaming response: %s", resp.Content.Parts[0].Text)
		return
	}
	t.Fatal("no response yielded")
}

func TestIntegration_Streaming(t *testing.T) {
	baseURL := os.Getenv("ANTHROPIC_TEST_URL")
	apiKey := os.Getenv("ANTHROPIC_TEST_KEY")
	if baseURL == "" || apiKey == "" {
		t.Skip("跳过集成测试：未设置 ANTHROPIC_TEST_URL / ANTHROPIC_TEST_KEY")
	}

	m := NewAnthropicModel("claude-opus-4-6", apiKey, baseURL, http.DefaultClient)
	req := &model.LLMRequest{
		Contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Reply with exactly: PONG"}}},
		},
		Config: &genai.GenerateContentConfig{
			MaxOutputTokens: 32,
		},
	}

	ctx := context.Background()
	var gotFinal bool
	for resp, err := range m.GenerateContent(ctx, req, true) {
		if err != nil {
			t.Fatalf("streaming error: %v", err)
		}
		if resp != nil && resp.TurnComplete {
			gotFinal = true
			t.Logf("streaming final: %d parts", len(resp.Content.Parts))
		}
	}
	if !gotFinal {
		t.Error("never received TurnComplete response")
	}
}
