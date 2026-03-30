package openai

import (
	"strings"

	"ds2api/internal/prompt"
)

func normalizeOpenAIMessagesForPrompt(raw []any, traceID string) []map[string]any {
	_ = traceID
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(asString(msg["role"])))
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
			out = append(out, map[string]any{
				"role":    "tool",
				"content": content,
			})
		case "user", "system", "developer":
			out = append(out, map[string]any{
				"role":    normalizeOpenAIRoleForPrompt(role),
				"content": normalizeOpenAIContentForPrompt(msg["content"]),
			})
		default:
			content := normalizeOpenAIContentForPrompt(msg["content"])
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
	content := strings.TrimSpace(normalizeOpenAIContentForPrompt(msg["content"]))
	toolHistory := prompt.FormatToolCallsForPrompt(msg["tool_calls"])
	switch {
	case content == "" && toolHistory == "":
		return ""
	case content == "":
		return toolHistory
	case toolHistory == "":
		return content
	default:
		return content + "\n\n" + toolHistory
	}
}

func buildToolContentForPrompt(msg map[string]any) string {
	content := normalizeOpenAIContentForPrompt(msg["content"])
	if strings.TrimSpace(content) == "" {
		return "null"
	}
	return content
}

func normalizeOpenAIContentForPrompt(v any) string {
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
