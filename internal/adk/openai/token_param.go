package openai

import "strings"

const (
	TokenParamModeAuto                = "auto"
	TokenParamModeMaxTokens           = "max_tokens"
	TokenParamModeMaxCompletionTokens = "max_completion_tokens"
)

func ResolveTokenParamMode(modelName string, mode string) string {
	switch normalizeTokenParamMode(mode) {
	case TokenParamModeMaxTokens:
		return TokenParamModeMaxTokens
	case TokenParamModeMaxCompletionTokens:
		return TokenParamModeMaxCompletionTokens
	default:
		if isReasoningStyleModel(modelName) {
			return TokenParamModeMaxCompletionTokens
		}
		return TokenParamModeMaxTokens
	}
}

func normalizeTokenParamMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case TokenParamModeMaxTokens:
		return TokenParamModeMaxTokens
	case TokenParamModeMaxCompletionTokens:
		return TokenParamModeMaxCompletionTokens
	default:
		return TokenParamModeAuto
	}
}

func isReasoningStyleModel(modelName string) bool {
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	return strings.HasPrefix(modelName, "gpt-5") ||
		strings.HasPrefix(modelName, "o1") ||
		strings.HasPrefix(modelName, "o3") ||
		strings.HasPrefix(modelName, "o4")
}
