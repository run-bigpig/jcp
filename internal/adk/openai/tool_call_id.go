package openai

import (
	"fmt"
	"regexp"
	"strings"
)

var toolCallIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func ensureFunctionCallID(id string, fallback string, index int) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}

	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		sanitized := toolCallIDSanitizer.ReplaceAllString(fallback, "_")
		sanitized = strings.Trim(sanitized, "_")
		if sanitized != "" {
			return "fc_" + sanitized
		}
	}
	return fmt.Sprintf("fc_auto_%d", index)
}
