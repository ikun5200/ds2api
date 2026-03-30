package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"ds2api/internal/prompt"
	"ds2api/internal/util"
)

func normalizeClaudeMessages(messages []any) []any {
	out := make([]any, 0, len(messages))
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", msg["role"])))
		switch content := msg["content"].(type) {
		case []any:
			textParts := make([]string, 0, len(content))
			flushText := func() {
				if len(textParts) == 0 {
					return
				}
				out = append(out, map[string]any{
					"role":    role,
					"content": strings.Join(textParts, "\n"),
				})
				textParts = textParts[:0]
			}
			for _, block := range content {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				typeStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", b["type"])))
				switch typeStr {
				case "text":
					if t, ok := b["text"].(string); ok {
						textParts = append(textParts, t)
					}
				case "tool_use":
					if role == "assistant" {
						flushText()
						if toolMsg := normalizeClaudeToolUseToAssistant(b); toolMsg != nil {
							out = append(out, toolMsg)
						}
						continue
					}
					if raw := strings.TrimSpace(formatClaudeUnknownBlockForPrompt(b)); raw != "" {
						textParts = append(textParts, raw)
					}
				case "tool_result":
					flushText()
					if toolMsg := normalizeClaudeToolResultToToolMessage(b); toolMsg != nil {
						out = append(out, toolMsg)
					}
				default:
					if raw := strings.TrimSpace(formatClaudeUnknownBlockForPrompt(b)); raw != "" {
						textParts = append(textParts, raw)
					}
				}
			}
			flushText()
		default:
			copied := cloneMap(msg)
			out = append(out, copied)
		}
	}
	return out
}

func buildClaudeToolPrompt(tools []any) string {
	toolSchemas := make([]string, 0, len(tools))
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, desc, schemaObj := extractClaudeToolMeta(m)
		if name == "" {
			continue
		}
		names = append(names, name)
		schema, _ := json.Marshal(schemaObj)
		toolSchemas = append(toolSchemas, fmt.Sprintf("Tool: %s\nDescription: %s\nParameters: %s", name, desc, schema))
	}
	if len(toolSchemas) == 0 {
		return ""
	}
	return "You have access to these tools:\n\n" +
		strings.Join(toolSchemas, "\n\n") + "\n\n" +
		util.BuildToolCallInstructions(names)
}

func formatClaudeToolResultForPrompt(block map[string]any) string {
	if block == nil {
		return ""
	}
	payload := map[string]any{
		"type":    "tool_result",
		"content": block["content"],
	}
	if toolCallID := strings.TrimSpace(fmt.Sprintf("%v", block["tool_use_id"])); toolCallID != "" {
		payload["tool_call_id"] = toolCallID
	} else if toolCallID := strings.TrimSpace(fmt.Sprintf("%v", block["tool_call_id"])); toolCallID != "" {
		payload["tool_call_id"] = toolCallID
	}
	if name := strings.TrimSpace(fmt.Sprintf("%v", block["name"])); name != "" {
		payload["name"] = name
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", payload))
	}
	return string(b)
}

func normalizeClaudeToolUseToAssistant(block map[string]any) map[string]any {
	if block == nil {
		return nil
	}
	name := strings.TrimSpace(fmt.Sprintf("%v", block["name"]))
	if name == "" {
		return nil
	}
	callID := strings.TrimSpace(fmt.Sprintf("%v", block["id"]))
	if callID == "" {
		callID = strings.TrimSpace(fmt.Sprintf("%v", block["tool_use_id"]))
	}
	if callID == "" {
		callID = "call_claude"
	}
	arguments := block["input"]
	if arguments == nil {
		arguments = map[string]any{}
	}
	argsJSON, err := json.Marshal(arguments)
	if err != nil || len(argsJSON) == 0 {
		argsJSON = []byte("{}")
	}
	toolCalls := []any{
		map[string]any{
			"id":   callID,
			"type": "function",
			"function": map[string]any{
				"name":      name,
				"arguments": string(argsJSON),
			},
		},
	}
	return map[string]any{
		"role":       "assistant",
		"content":    prompt.FormatToolCallsForPrompt(toolCalls),
		"tool_calls": toolCalls,
	}
}

func normalizeClaudeToolResultToToolMessage(block map[string]any) map[string]any {
	if block == nil {
		return nil
	}
	toolCallID := strings.TrimSpace(fmt.Sprintf("%v", block["tool_use_id"]))
	if toolCallID == "" {
		toolCallID = strings.TrimSpace(fmt.Sprintf("%v", block["tool_call_id"]))
	}
	if toolCallID == "" {
		toolCallID = "call_claude"
	}
	out := map[string]any{
		"role":         "tool",
		"tool_call_id": toolCallID,
		"content":      normalizeClaudeToolResultContent(block["content"]),
	}
	if name := strings.TrimSpace(fmt.Sprintf("%v", block["name"])); name != "" {
		out["name"] = name
	}
	return out
}

func normalizeClaudeToolResultContent(content any) any {
	if text, ok := content.(string); ok {
		return text
	}
	payload := map[string]any{
		"type":    "tool_result",
		"content": content,
	}
	b, err := json.Marshal(sanitizeClaudeBlockForPrompt(payload))
	if err != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", content))
	}
	return string(b)
}

func formatClaudeBlockRaw(block map[string]any) string {
	if block == nil {
		return ""
	}
	b, err := json.Marshal(block)
	if err != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", block))
	}
	return string(b)
}

func hasSystemMessage(messages []any) bool {
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if ok && msg["role"] == "system" {
			return true
		}
	}
	return false
}

func extractClaudeToolNames(tools []any) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		m, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, _, _ := extractClaudeToolMeta(m)
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func extractClaudeToolMeta(m map[string]any) (string, string, any) {
	name, _ := m["name"].(string)
	desc, _ := m["description"].(string)
	schemaObj := m["input_schema"]
	if schemaObj == nil {
		schemaObj = m["parameters"]
	}

	if fn, ok := m["function"].(map[string]any); ok {
		if strings.TrimSpace(name) == "" {
			name, _ = fn["name"].(string)
		}
		if strings.TrimSpace(desc) == "" {
			desc, _ = fn["description"].(string)
		}
		if schemaObj == nil {
			if v, ok := fn["input_schema"]; ok {
				schemaObj = v
			}
		}
		if schemaObj == nil {
			if v, ok := fn["parameters"]; ok {
				schemaObj = v
			}
		}
	}
	return strings.TrimSpace(name), strings.TrimSpace(desc), schemaObj
}

func toMessageMaps(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func extractMessageContent(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, it := range x {
			parts = append(parts, fmt.Sprintf("%v", it))
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", x)
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
