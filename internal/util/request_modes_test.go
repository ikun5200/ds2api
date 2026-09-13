package util

import "testing"

func TestExplicitModeBooleansAndExtraBody(t *testing.T) {
	for _, mode := range []struct {
		field   string
		resolve func(map[string]any, bool) bool
	}{
		{"thinking_enabled", ResolveThinkingEnabled},
		{"search_enabled", ResolveSearchEnabled},
	} {
		t.Run(mode.field, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				req := map[string]any{mode.field: enabled, "extra_body": map[string]any{mode.field: !enabled}}
				if mode.resolve(req, !enabled) != enabled {
					t.Fatalf("top-level %s=%v must override extra_body and default", mode.field, enabled)
				}
				delete(req, mode.field)
				if mode.resolve(req, enabled) != !enabled {
					t.Fatalf("extra_body %s must override default", mode.field)
				}
			}
			if mode.resolve(nil, true) != true || mode.resolve(nil, false) != false {
				t.Fatal("missing option must preserve default")
			}
		})
	}
}

func TestThinkingEnabledOverridesLegacyThinking(t *testing.T) {
	if ResolveThinkingEnabled(map[string]any{
		"thinking_enabled": false,
		"thinking":         map[string]any{"type": "enabled"},
		"reasoning_effort": "high",
	}, true) {
		t.Fatal("thinking_enabled=false must remain disabled")
	}
}
