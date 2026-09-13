package promptcompat

import "testing"

func TestFlashModesReachCompletionAcrossOpenAISurfaces(t *testing.T) {
	for _, surface := range []string{"chat", "responses"} {
		for _, thinking := range []bool{false, true} {
			for _, search := range []bool{false, true} {
				req := map[string]any{
					"model":            "deepseek-flash",
					"thinking_enabled": thinking,
					"search_enabled":   search,
					"messages":         []any{map[string]any{"role": "user", "content": "hello"}},
				}
				var std StandardRequest
				var err error
				if surface == "responses" {
					std, err = NormalizeOpenAIResponsesRequest(nil, req, "")
				} else {
					std, err = NormalizeOpenAIChatRequest(nil, req, "")
				}
				if err != nil {
					t.Fatal(err)
				}
				payload := std.CompletionPayload("session")
				if payload["thinking_enabled"] != thinking || payload["search_enabled"] != search || payload["model_type"] != "default" {
					t.Fatalf("%s thinking=%v search=%v: payload=%#v", surface, thinking, search, payload)
				}
				if action, ok := payload["action"]; !ok || action != nil || payload["preempt"] != false {
					t.Fatalf("missing website completion defaults: %#v", payload)
				}
			}
		}
	}
}

func TestRequestModesOverrideLegacySearchAndRespectNoThinking(t *testing.T) {
	thinking, search := ResolveRequestModes(map[string]any{
		"thinking_enabled": true,
		"search_enabled":   false,
	}, "deepseek-v4-pro-search-nothinking")
	if thinking || search {
		t.Fatalf("expected explicit search=false and forced thinking=false, got %v/%v", thinking, search)
	}
}
