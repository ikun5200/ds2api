package gemini

import (
	"encoding/json"

	"ds2api/internal/promptcompat"
)

func preserveGeminiProxyAttachments(translated []byte, original map[string]any) []byte {
	var request map[string]any
	if json.Unmarshal(translated, &request) != nil {
		return translated
	}
	if refs := promptcompat.CollectOpenAIRefFileIDs(original); len(refs) > 0 {
		request["ref_file_ids"] = refs
	}
	normalized := geminiMessagesFromRequest(original)
	for _, raw := range normalized {
		message, _ := raw.(map[string]any)
		if attachments, _ := message["attachments"].([]any); len(attachments) > 0 {
			// Native normalization preserves snake-case and tool-result parts
			// that the external translator drops. Shared inputfiles handles
			// their upload and validation after this adapter boundary.
			request["messages"] = normalized
			break
		}
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return translated
	}
	return encoded
}
