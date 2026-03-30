package util

import (
	"regexp"
	"strings"
)

var toolCallPattern = regexp.MustCompile(`\{\s*["']tool_calls["']\s*:\s*\[(.*?)\]\s*\}`)
var fencedJSONPattern = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")
var fencedCodeBlockPattern = regexp.MustCompile("(?s)```[\\s\\S]*?```")
var markupToolSyntaxPattern = regexp.MustCompile(`(?i)<(?:(?:[a-z0-9_:-]+:)?(?:tool_call|function_call|invoke)\b|(?:[a-z0-9_:-]+:)?function_calls\b|(?:[a-z0-9_:-]+:)?tool_use\b)`)

func buildToolCallCandidates(text string) []string {
	trimmed := strings.TrimSpace(text)
	candidates := []string{trimmed}

	// fenced code block candidates: ```json ... ```
	for _, match := range fencedJSONPattern.FindAllStringSubmatch(trimmed, -1) {
		if len(match) >= 2 {
			candidates = append(candidates, strings.TrimSpace(match[1]))
		}
	}

	// best-effort extraction around tool call keywords in mixed text payloads.
	candidates = append(candidates, extractToolCallObjects(trimmed)...)

	// best-effort object slice: from first '{' to last '}'
	first := strings.Index(trimmed, "{")
	last := strings.LastIndex(trimmed, "}")
	if first >= 0 && last > first {
		candidates = append(candidates, strings.TrimSpace(trimmed[first:last+1]))
	}
	// best-effort array slice: from first '[' to last ']'
	firstArr := strings.Index(trimmed, "[")
	lastArr := strings.LastIndex(trimmed, "]")
	if firstArr >= 0 && lastArr > firstArr {
		candidates = append(candidates, strings.TrimSpace(trimmed[firstArr:lastArr+1]))
	}

	// legacy regex extraction fallback
	if m := toolCallPattern.FindStringSubmatch(trimmed); len(m) >= 2 {
		candidates = append(candidates, "{"+`"tool_calls":[`+m[1]+"]}")
	}

	uniq := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		uniq = append(uniq, c)
	}
	return uniq
}

func extractToolCallObjects(text string) []string {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	out := []string{}
	offset := 0
	keywords := []string{"tool_calls", "\"function\"", "function.name:"}
	for {
		bestIdx := -1
		matchedKeyword := ""
		for _, kw := range keywords {
			idx := strings.Index(lower[offset:], kw)
			if idx >= 0 {
				absIdx := offset + idx
				if bestIdx < 0 || absIdx < bestIdx {
					bestIdx = absIdx
					matchedKeyword = kw
				}
			}
		}

		if bestIdx < 0 {
			break
		}

		idx := bestIdx
		// Avoid backtracking too far to prevent OOM on malicious or very long strings
		searchLimit := idx - 2000
		if searchLimit < offset {
			searchLimit = offset
		}

		start := strings.LastIndex(text[searchLimit:idx], "{")
		if start >= 0 {
			start += searchLimit
		}

		if start < 0 {
			offset = idx + len(matchedKeyword)
			continue
		}

		foundObj := false
		for start >= searchLimit {
			candidate, end, ok := extractJSONObject(text, start)
			if ok {
				// Move forward to avoid repeatedly matching the same object.
				offset = end
				out = append(out, strings.TrimSpace(candidate))
				foundObj = true
				break
			}
			// Try previous '{'
			if start > searchLimit {
				prevStart := strings.LastIndex(text[searchLimit:start], "{")
				if prevStart >= 0 {
					start = searchLimit + prevStart
					continue
				}
			}
			break
		}

		if !foundObj {
			offset = idx + len(matchedKeyword)
		}
	}
	return out
}

func extractJSONObject(text string, start int) (string, int, bool) {
	if start < 0 || start >= len(text) || text[start] != '{' {
		return "", 0, false
	}
	depth := 0
	quote := byte(0)
	escaped := false
	// Limit scan length to avoid OOM on unclosed objects
	maxLen := start + 50000
	if maxLen > len(text) {
		maxLen = len(text)
	}
	for i := start; i < maxLen; i++ {
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
				return text[start : i+1], i + 1, true
			}
		}
	}
	return "", 0, false
}

func looksLikeToolExampleContext(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	return strings.Contains(t, "```")
}

func shouldSkipToolCallParsingForCodeFenceExample(text string) bool {
	if !looksLikeToolCallSyntax(text) {
		return false
	}
	stripped := strings.TrimSpace(stripFencedCodeBlocks(text))
	return !looksLikeToolCallSyntax(stripped)
}

func looksLikeMarkupToolSyntax(text string) bool {
	return markupToolSyntaxPattern.MatchString(text)
}

func stripFencedCodeBlocks(text string) string {
	if text == "" {
		return ""
	}
	return fencedCodeBlockPattern.ReplaceAllString(text, " ")
}
