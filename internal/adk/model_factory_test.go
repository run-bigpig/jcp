package adk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/run-bigpig/jcp/internal/models"
)

func TestNormalizeAnthropicBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantURL string
	}{
		{name: "empty uses official", input: "", wantURL: "https://api.anthropic.com"},
		{name: "trims trailing slash", input: "https://api.anthropic.com/", wantURL: "https://api.anthropic.com"},
		{name: "strips v1 suffix", input: "https://api.anthropic.com/v1", wantURL: "https://api.anthropic.com"},
		{name: "strips v1 suffix with slash", input: "https://proxy.example.com/path/v1/", wantURL: "https://proxy.example.com/path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeAnthropicBaseURL(tc.input)
			if got != tc.wantURL {
				t.Fatalf("normalizeAnthropicBaseURL(%q) = %q, want %q", tc.input, got, tc.wantURL)
			}
		})
	}
}

func TestTestOpenAIConnection_RetriesAlternateTokenParam(t *testing.T) {
	requestBodies := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestBodies = append(requestBodies, body)

		if _, ok := body["max_tokens"]; ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"this model is not supported MaxTokens, please use MaxCompletionTokens"}}`))
			return
		}
		if got := body["max_completion_tokens"]; got != float64(1) {
			t.Fatalf("max_completion_tokens = %v, want 1", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	defer server.Close()

	factory := NewModelFactory()
	config := &models.AIConfig{
		Provider:       models.AIProviderOpenAI,
		BaseURL:        server.URL,
		APIKey:         "test-key",
		ModelName:      "prod-assistant",
		TokenParamMode: models.OpenAITokenParamAuto,
	}

	if err := factory.TestConnection(context.Background(), config); err != nil {
		t.Fatalf("TestConnection returned error: %v", err)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(requestBodies))
	}
	if _, ok := requestBodies[0]["max_tokens"]; !ok {
		t.Fatalf("first request should use max_tokens: %+v", requestBodies[0])
	}
	if _, ok := requestBodies[1]["max_completion_tokens"]; !ok {
		t.Fatalf("second request should use max_completion_tokens: %+v", requestBodies[1])
	}
}

func TestDetectOpenAISystemRole_RetriesAlternateTokenParam(t *testing.T) {
	requestBodies := make([]map[string]any, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		requestBodies = append(requestBodies, body)

		if _, ok := body["max_tokens"]; ok {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"unsupported parameter: max_tokens, please use max_completion_tokens"}}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"SYS_PROBE_7X3K"}}]}`))
	}))
	defer server.Close()

	factory := NewModelFactory()
	config := &models.AIConfig{
		Provider:       models.AIProviderOpenAI,
		BaseURL:        server.URL,
		APIKey:         "test-key",
		ModelName:      "prod-assistant",
		TokenParamMode: models.OpenAITokenParamAuto,
	}

	if unsupported := factory.DetectSystemRoleSupport(context.Background(), config); unsupported {
		t.Fatal("DetectSystemRoleSupport returned unsupported, want supported after retry")
	}
	if len(requestBodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(requestBodies))
	}
	if _, ok := requestBodies[1]["max_completion_tokens"]; !ok {
		t.Fatalf("second request should use max_completion_tokens: %+v", requestBodies[1])
	}
}
