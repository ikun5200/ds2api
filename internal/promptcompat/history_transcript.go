package promptcompat

import (
	"fmt"
	"strings"
)

const CurrentInputContextFilename = "HISTORY.txt"

const historyTranscriptTitle = "# " + CurrentInputContextFilename
const historyTranscriptSummary = "Prior conversation history and tool progress."

func BuildOpenAIHistoryTranscript(messages []any) string {
	return buildOpenAIHistoryTranscript(messages)
}

func BuildOpenAICurrentUserInputTranscript(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return buildOpenAIHistoryTranscript([]any{
		map[string]any{"role": "user", "content": text},
	})
}

func BuildOpenAICurrentInputContextTranscript(messages []any) string {
	return buildOpenAIHistoryTranscript(messages)
}

func buildOpenAIHistoryTranscript(messages []any) string {
	if len(messages) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(historyTranscriptTitle)
	b.WriteString("\n")
	b.WriteString(historyTranscriptSummary)
	b.WriteString("\n\n")

	entry := 0
	callNameByID := map[string]string{}
	for _, raw := range messages {
		msg, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role := normalizeOpenAIRoleForPrompt(strings.ToLower(strings.TrimSpace(asString(msg["role"]))))
		if role == "assistant" {
			registerAssistantToolCallNames(msg, callNameByID)
		}
		content := strings.TrimSpace(buildOpenAIHistoryEntry(role, msg, callNameByID))
		if content == "" {
			continue
		}
		entry++
		fmt.Fprintf(&b, "=== %d. %s ===\n%s\n\n", entry, strings.ToUpper(roleLabelForHistory(role)), content)
	}

	transcript := strings.TrimSpace(b.String())
	if transcript == "" {
		return ""
	}
	return transcript + "\n"
}

func buildOpenAIHistoryEntry(role string, msg map[string]any, callNameByID map[string]string) string {
	switch role {
	case "assistant":
		return strings.TrimSpace(buildAssistantContentForPrompt(msg))
	case "tool", "function":
		return strings.TrimSpace(buildToolHistoryContent(msg, callNameByID))
	case "system", "user":
		return strings.TrimSpace(NormalizeOpenAIContentForPrompt(msg["content"]))
	default:
		return strings.TrimSpace(NormalizeOpenAIContentForPrompt(msg["content"]))
	}
}

func buildToolHistoryContent(msg map[string]any, callNameByID map[string]string) string {
	content := strings.TrimSpace(NormalizeOpenAIContentForPrompt(msg["content"]))
	name := strings.TrimSpace(asString(msg["name"]))
	callID := strings.TrimSpace(asString(msg["tool_call_id"]))
	if name == "" && callID != "" {
		name = strings.TrimSpace(callNameByID[callID])
	}
	parts := make([]string, 0, 2)
	if name != "" {
		parts = append(parts, "name="+name)
	}
	if callID != "" {
		parts = append(parts, "tool_call_id="+callID)
	}
	header := ""
	if len(parts) > 0 {
		header = "[" + strings.Join(parts, " ") + "]"
	}
	switch {
	case header != "" && content != "":
		return header + "\n" + content
	case header != "":
		return header
	default:
		return content
	}
}

// registerAssistantToolCallNames records the assistant tool call id -> name
// mapping so later tool messages can keep an explicit name even when the
// client only sent tool_call_id.
func registerAssistantToolCallNames(msg map[string]any, out map[string]string) {
	if msg == nil || out == nil {
		return
	}
	calls, _ := msg["tool_calls"].([]any)
	for _, raw := range calls {
		call, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(asString(call["id"]))
		if id == "" {
			continue
		}
		name := strings.TrimSpace(asString(call["name"]))
		if name == "" {
			if fn, ok := call["function"].(map[string]any); ok {
				name = strings.TrimSpace(asString(fn["name"]))
			}
		}
		if name != "" {
			out[id] = name
		}
	}
}

func roleLabelForHistory(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "function":
		return "tool"
	case "":
		return "unknown"
	default:
		return role
	}
}
