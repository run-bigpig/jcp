package adk

import "testing"

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
