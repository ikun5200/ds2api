package claude

import (
	"encoding/json"
	"testing"
)

func TestClaudeFlashModesMatchProxyPreparation(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		for _, search := range []bool{false, true} {
			req := map[string]any{
				"model":      "deepseek-flash",
				"messages":   []any{map[string]any{"role": "user", "content": "hello"}},
				"extra_body": map[string]any{"thinking_enabled": thinking, "search_enabled": search},
			}
			normalized, err := normalizeClaudeRequest(mockClaudeConfig{}, req)
			if err != nil {
				t.Fatal(err)
			}
			if normalized.Standard.Thinking != thinking || normalized.Standard.Search != search {
				t.Fatalf("unexpected modes: %v/%v", normalized.Standard.Thinking, normalized.Standard.Search)
			}
			raw, exposed := applyClaudeThinkingPolicyToOpenAIRequest([]byte(`{"model":"deepseek-flash"}`), req)
			var translated map[string]any
			if err := json.Unmarshal(raw, &translated); err != nil {
				t.Fatal(err)
			}
			if translated["thinking_enabled"] != thinking || translated["search_enabled"] != search || exposed != thinking {
				t.Fatalf("proxy lost modes: %s", raw)
			}
		}
	}
}
