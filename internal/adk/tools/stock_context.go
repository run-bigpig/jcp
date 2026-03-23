package tools

import (
	"regexp"
	"strings"

	"google.golang.org/adk/tool"
)

var (
	stockCodePattern  = regexp.MustCompile(`(?i)\b(?:sh|sz|bj)\d{6}\b`)
	sixDigitCodeRegex = regexp.MustCompile(`\b\d{6}\b`)
)

func stockCodeFromToolContext(ctx tool.Context) string {
	if ctx == nil {
		return ""
	}
	userContent := ctx.UserContent()
	if userContent == nil {
		return ""
	}

	var sb strings.Builder
	for _, part := range userContent.Parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(text)
	}
	return detectStockCodeFromText(sb.String())
}

func detectStockCodeFromText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if match := stockCodePattern.FindString(text); match != "" {
		return strings.ToLower(match)
	}
	if digits := sixDigitCodeRegex.FindString(text); digits != "" {
		return inferStockCodePrefix(digits)
	}
	return ""
}

func normalizeStockSymbol(raw string) string {
	candidate := strings.ToLower(strings.TrimSpace(raw))
	if candidate == "" {
		return ""
	}
	if strings.HasPrefix(candidate, "s_") {
		candidate = strings.TrimPrefix(candidate, "s_")
	}
	if match := stockCodePattern.FindString(candidate); match != "" {
		return strings.ToLower(match)
	}
	if digits := sixDigitCodeRegex.FindString(candidate); digits != "" {
		return inferStockCodePrefix(digits)
	}
	return ""
}

func normalizeStockSymbolList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))

	for _, value := range values {
		for _, segment := range splitCodeCandidates(value) {
			code := normalizeStockSymbol(segment)
			if code == "" {
				continue
			}
			if _, ok := seen[code]; ok {
				continue
			}
			seen[code] = struct{}{}
			result = append(result, code)
		}
	}
	return result
}

func resolveStockCodeFromCandidates(ctx tool.Context, candidates ...string) string {
	if codes := normalizeStockSymbolList(candidates); len(codes) > 0 {
		return codes[0]
	}
	if code := stockCodeFromToolContext(ctx); code != "" {
		return code
	}
	return ""
}

func splitCodeCandidates(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/', '\\', ' ', '\n', '\t':
			return true
		default:
			return false
		}
	})
}

func inferStockCodePrefix(code string) string {
	if len(code) != 6 {
		return ""
	}
	switch code[0] {
	case '6':
		return "sh" + code
	case '0', '3':
		return "sz" + code
	case '4', '8':
		return "bj" + code
	default:
		return ""
	}
}
