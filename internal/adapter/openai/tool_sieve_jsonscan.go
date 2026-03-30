package openai

import "strings"

func extractJSONObjectFrom(text string, start int) (string, int, bool) {
	if start < 0 || start >= len(text) || text[start] != '{' {
		return "", 0, false
	}
	depth := 0
	quote := byte(0)
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			continue
		}
		if ch == '{' {
			depth++
			continue
		}
		if ch == '}' {
			depth--
			if depth == 0 {
				end := i + 1
				return text[start:end], end, true
			}
		}
	}
	return "", 0, false
}

func trimWrappingJSONFence(prefix, suffix string) (string, string) {
	trimmedPrefix := strings.TrimRight(prefix, " \t\r\n")
	fenceIdx := strings.LastIndex(trimmedPrefix, "```")
	if fenceIdx < 0 {
		return prefix, suffix
	}
	// Only strip when the trailing fence in prefix behaves like an opening fence.
	// A legitimate closing fence before a standalone tool JSON must be preserved.
	if strings.Count(trimmedPrefix[:fenceIdx+3], "```")%2 == 0 {
		return prefix, suffix
	}
	fenceHeader := strings.TrimSpace(trimmedPrefix[fenceIdx+3:])
	if fenceHeader != "" && !strings.EqualFold(fenceHeader, "json") {
		return prefix, suffix
	}

	trimmedSuffix := strings.TrimLeft(suffix, " \t\r\n")
	if !strings.HasPrefix(trimmedSuffix, "```") {
		return prefix, suffix
	}
	consumedLeading := len(suffix) - len(trimmedSuffix)
	return trimmedPrefix[:fenceIdx], suffix[consumedLeading+3:]
}

func openFenceStartBefore(s string, pos int) (int, bool) {
	if pos <= 0 || pos > len(s) {
		return -1, false
	}
	segment := s[:pos]
	lastFence := strings.LastIndex(segment, "```")
	if lastFence < 0 {
		return -1, false
	}
	if strings.Count(segment, "```")%2 == 1 {
		return lastFence, true
	}
	return -1, false
}
