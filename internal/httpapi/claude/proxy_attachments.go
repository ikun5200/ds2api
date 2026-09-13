package claude

import (
	"encoding/json"
	"strings"

	"ds2api/internal/promptcompat"
)

func preserveClaudeProxyAttachments(translated []byte, original map[string]any) []byte {
	var request map[string]any
	if json.Unmarshal(translated, &request) != nil {
		return translated
	}
	if refs := promptcompat.CollectOpenAIRefFileIDs(original); len(refs) > 0 {
		request["ref_file_ids"] = refs
	}
	rawMessages, _ := original["messages"].([]any)
	normalized := normalizeClaudeMessages(rawMessages)
	hasAttachments := false
	for _, raw := range normalized {
		message, _ := raw.(map[string]any)
		attachments, _ := message["attachments"].([]any)
		hasAttachments = hasAttachments || len(attachments) > 0
	}
	if hasAttachments {
		// The external translator omits documents and file IDs. Use the same
		// normalized attachment messages as the native path, retaining its
		// system conversion and tool definitions at the protocol boundary.
		var messages []any
		translatedMessages, _ := request["messages"].([]any)
		for _, raw := range translatedMessages {
			message, _ := raw.(map[string]any)
			if role := strings.ToLower(safeStringValue(message["role"])); role == "system" || role == "developer" {
				messages = append(messages, raw)
			}
		}
		for _, raw := range normalized {
			message, _ := raw.(map[string]any)
			if role := strings.ToLower(safeStringValue(message["role"])); role != "system" && role != "developer" {
				messages = append(messages, raw)
			}
		}
		request["messages"] = messages
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return translated
	}
	return encoded
}
