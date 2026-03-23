package openai

import "strings"

func normalizeLogText(input string, max int) string {
	if max <= 0 {
		max = 256
	}
	text := strings.TrimSpace(input)
	if text == "" {
		return ""
	}
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > max {
		return text[:max] + "...(truncated)"
	}
	return text
}
