package promptcompat

import (
	"strings"

	"ds2api/internal/prompt"
	"ds2api/internal/toolcall"
)

const assistantReasoningLabel = "reasoning_content"

func NormalizeOpenAIMessagesForPrompt(raw []any, traceID string) []map[string]any {
	_ = traceID
	out := make([]map[string]any, 0, len(raw))
	callNameByID := map[string]string{}
	for _, item := range raw {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(asString(msg["role"])))
		if role == "assistant" {
			registerAssistantToolCallNames(msg, callNameByID)
		}
		switch role {
		case "assistant":
			content := buildAssistantContentForPrompt(msg)
			if content == "" {
				continue
			}
			out = append(out, map[string]any{
				"role":    "assistant",
				"content": content,
			})
		case "tool", "function":
			content := buildToolContentForPrompt(msg)
			outMsg := map[string]any{
				"role":    "tool",
				"content": content,
			}
			callID := strings.TrimSpace(asString(msg["tool_call_id"]))
			name := strings.TrimSpace(asString(msg["name"]))
			if name == "" && callID != "" {
				name = strings.TrimSpace(callNameByID[callID])
			}
			if name != "" {
				outMsg["name"] = name
			}
			if callID != "" {
				outMsg["tool_call_id"] = callID
			}
			out = append(out, outMsg)
		case "user", "system", "developer":
			out = append(out, map[string]any{
				"role":    normalizeOpenAIRoleForPrompt(role),
				"content": NormalizeOpenAIContentForPrompt(msg["content"]),
			})
		default:
			content := NormalizeOpenAIContentForPrompt(msg["content"])
			if content == "" {
				continue
			}
			if role == "" {
				role = "user"
			}
			out = append(out, map[string]any{
				"role":    normalizeOpenAIRoleForPrompt(role),
				"content": content,
			})
		}
	}
	return out
}

func buildAssistantContentForPrompt(msg map[string]any) string {
	content := strings.TrimSpace(NormalizeOpenAIContentForPrompt(msg["content"]))
	reasoning := strings.TrimSpace(normalizeOpenAIReasoningContentForPrompt(msg["reasoning_content"]))
	if reasoning == "" {
		reasoning = strings.TrimSpace(extractOpenAIReasoningContentFromMessage(msg["content"]))
	}
	toolHistory := prompt.FormatToolCallsForPrompt(assistantToolCallsFromMessage(msg))
	if toolHistory == "" {
		content = normalizeAssistantToolMarkupContentForPrompt(content)
	}
	parts := make([]string, 0, 3)
	if reasoning != "" {
		parts = append(parts, formatPromptLabeledBlock(assistantReasoningLabel, reasoning))
	}
	if content != "" {
		parts = append(parts, content)
	}
	if toolHistory != "" {
		parts = append(parts, toolHistory)
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return strings.Join(parts, "\n\n")
	}
}

func assistantToolCallsFromMessage(msg map[string]any) []any {
	if calls, ok := msg["tool_calls"].([]any); ok && len(calls) > 0 {
		return calls
	}
	if legacy, ok := msg["function_call"].(map[string]any); ok && len(legacy) > 0 {
		return []any{legacy}
	}
	return nil
}

func normalizeAssistantToolMarkupContentForPrompt(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || !isStandaloneAssistantToolMarkupBlock(trimmed) {
		return content
	}
	parsed := toolcall.ParseStandaloneToolCallsDetailed(trimmed, nil)
	if len(parsed.Calls) == 0 {
		return content
	}
	raw := make([]any, 0, len(parsed.Calls))
	for _, call := range parsed.Calls {
		raw = append(raw, map[string]any{
			"name":  call.Name,
			"input": call.Input,
		})
	}
	if formatted := prompt.FormatToolCallsForPrompt(raw); formatted != "" {
		return formatted
	}
	return content
}

func isStandaloneAssistantToolMarkupBlock(trimmed string) bool {
	tag, ok := toolcall.FindToolMarkupTagOutsideIgnored(trimmed, 0)
	if !ok || tag.Start != 0 || tag.Closing || tag.Name != "tool_calls" {
		return false
	}
	closeTag, ok := toolcall.FindMatchingToolMarkupClose(trimmed, tag)
	if !ok {
		return false
	}
	return strings.TrimSpace(trimmed[closeTag.End+1:]) == ""
}

func normalizeOpenAIReasoningContentForPrompt(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		return strings.Join(extractOpenAIReasoningPartsFromItems(x), "\n")
	case map[string]any:
		return extractOpenAIReasoningTextFromItem(x)
	default:
		return ""
	}
}

func extractOpenAIReasoningContentFromMessage(v any) string {
	switch x := v.(type) {
	case []any:
		return strings.Join(extractOpenAIReasoningPartsFromItems(x), "\n")
	case map[string]any:
		return extractOpenAIReasoningTextFromItem(x)
	default:
		return ""
	}
}

func extractOpenAIReasoningPartsFromItems(items []any) []string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if text := extractOpenAIReasoningTextFromItemMap(item); text != "" {
			parts = append(parts, text)
		}
	}
	return parts
}

func extractOpenAIReasoningTextFromItemMap(item any) string {
	m, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	return extractOpenAIReasoningTextFromItem(m)
}

func extractOpenAIReasoningTextFromItem(m map[string]any) string {
	if m == nil {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(asString(m["type"]))) {
	case "reasoning", "thinking":
		for _, key := range []string{"text", "thinking", "content"} {
			if text := strings.TrimSpace(asString(m[key])); text != "" {
				return text
			}
		}
	}
	return ""
}

func formatPromptLabeledBlock(label, text string) string {
	label = strings.TrimSpace(label)
	text = strings.TrimSpace(text)
	if label == "" {
		return text
	}
	return "[" + label + "]\n" + text + "\n[/" + label + "]"
}

func buildToolContentForPrompt(msg map[string]any) string {
	content := NormalizeOpenAIContentForPrompt(msg["content"])
	if strings.TrimSpace(content) == "" {
		return "null"
	}
	return content
}

func NormalizeOpenAIContentForPrompt(v any) string {
	return prompt.NormalizeContent(v)
}

func normalizeOpenAIRoleForPrompt(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "developer" {
		return "system"
	}
	return role
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
