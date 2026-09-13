package gemini

import (
	"encoding/json"
	"testing"
)

func TestGeminiNativeModesMatchProxyPreparation(t *testing.T) {
	for _, budget := range []int{-1, 0, 1024} {
		req := map[string]any{
			"contents":         []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "hello"}}}},
			"generationConfig": map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": budget}},
			"tools":            []any{map[string]any{"googleSearch": map[string]any{}}},
		}
		std, err := normalizeGeminiRequest(testGeminiConfig{}, "deepseek-flash", req, false)
		if err != nil {
			t.Fatal(err)
		}
		if std.Thinking != (budget != 0) || !std.Search {
			t.Fatalf("budget=%d: modes=%v/%v", budget, std.Thinking, std.Search)
		}
		var translated map[string]any
		if err := json.Unmarshal(applyGeminiThinkingPolicyToOpenAIRequest([]byte(`{"model":"deepseek-flash"}`), req), &translated); err != nil {
			t.Fatal(err)
		}
		if translated["thinking_enabled"] != std.Thinking || translated["search_enabled"] != std.Search {
			t.Fatalf("direct and proxy modes disagree: %#v", translated)
		}
	}
}

func TestGeminiExplicitModesOverrideNativeToolsAndBudget(t *testing.T) {
	req := map[string]any{
		"thinking_enabled": false,
		"search_enabled":   false,
		"generationConfig": map[string]any{"thinkingConfig": map[string]any{"thinkingBudget": -1}},
		"tools":            []any{map[string]any{"googleSearch": map[string]any{}}},
	}
	normalized := geminiModeRequest(req)
	if normalized["thinking_enabled"] != false || normalized["search_enabled"] != false {
		t.Fatalf("explicit modes were overridden: %#v", normalized)
	}
}
