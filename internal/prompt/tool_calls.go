package prompt

import (
	"encoding/json"
	"strings"
)

var promptXMLTextEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

// FormatToolCallsForPrompt renders a tool_calls slice into the canonical
// prompt-visible history block used across adapters.
func FormatToolCallsForPrompt(raw any) string {
	calls, ok := raw.([]any)
	if !ok || len(calls) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(calls))
	for _, item := range calls {
		call, ok := item.(map[string]any)
		if !ok {
			continue
		}
		block := formatToolCallForPrompt(call)
		if block != "" {
			blocks = append(blocks, block)
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	return "<tool_calls>\n" + strings.Join(blocks, "\n") + "\n</tool_calls>"
}

// StringifyToolCallArguments normalizes tool arguments into a compact string
// while preserving raw concatenated payloads when they already look like model
// output rather than a single JSON object.
func StringifyToolCallArguments(v any) string {
	switch x := v.(type) {
	case nil:
		return "{}"
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return "{}"
		}
		s = normalizeToolArgumentString(s)
		if s == "" {
			return "{}"
		}
		return s
	default:
		b, err := json.Marshal(x)
		if err != nil || len(b) == 0 {
			return "{}"
		}
		return string(b)
	}
}

func formatToolCallForPrompt(call map[string]any) string {
	if call == nil {
		return ""
	}

	name := strings.TrimSpace(asString(call["name"]))
	fn, _ := call["function"].(map[string]any)
	if name == "" && fn != nil {
		name = strings.TrimSpace(asString(fn["name"]))
	}
	if name == "" {
		return ""
	}

	argsRaw := call["arguments"]
	if argsRaw == nil {
		argsRaw = call["input"]
	}
	if argsRaw == nil && fn != nil {
		argsRaw = fn["arguments"]
		if argsRaw == nil {
			argsRaw = fn["input"]
		}
	}

	return "  <tool_call>\n" +
		"    <tool_name>" + escapeXMLText(name) + "</tool_name>\n" +
		"    <parameters>" + escapeXMLText(StringifyToolCallArguments(argsRaw)) + "</parameters>\n" +
		"  </tool_call>"
}

func normalizeToolArgumentString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if looksLikeConcatenatedJSON(trimmed) {
		// Keep the original payload to avoid silently rewriting model output.
		return raw
	}
	return trimmed
}

func looksLikeConcatenatedJSON(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "}{") || strings.Contains(trimmed, "][") {
		return true
	}
	dec := json.NewDecoder(strings.NewReader(trimmed))
	var first any
	if err := dec.Decode(&first); err != nil {
		return false
	}
	var second any
	return dec.Decode(&second) == nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func escapeXMLText(v string) string {
	if v == "" {
		return ""
	}
	return promptXMLTextEscaper.Replace(v)
}
