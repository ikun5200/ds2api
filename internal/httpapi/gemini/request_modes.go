package gemini

import "ds2api/internal/util"

// Gemini's native budget/search tool spellings are an adapter concern. The
// normalized booleans then follow the same mode policy as Chat/Responses/Claude.
func geminiModeRequest(req map[string]any) map[string]any {
	out := make(map[string]any, len(req)+2)
	for k, v := range req {
		out[k] = v
	}
	if _, ok := util.ResolveThinkingOverride(out); !ok {
		if enabled, ok := resolveGeminiThinkingOverride(req); ok {
			out["thinking_enabled"] = enabled
		}
	}
	if _, ok := util.ResolveSearchOverride(out); !ok {
		tools, _ := req["tools"].([]any)
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			for _, key := range []string{"googleSearch", "google_search", "googleSearchRetrieval", "google_search_retrieval"} {
				if _, ok := tool[key].(map[string]any); ok {
					out["search_enabled"] = true
				}
			}
		}
	}
	return out
}
