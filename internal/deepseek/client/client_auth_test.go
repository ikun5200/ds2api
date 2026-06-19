package client

import (
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestLoginDeviceIDUsesStableAccountDerivedValue(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_SEED", "test-seed")
	acc := config.Account{Email: "USER@example.com"}

	first := loginDeviceID(acc)
	second := loginDeviceID(acc)
	if first == "" {
		t.Fatal("expected non-empty device id")
	}
	if first != second {
		t.Fatalf("expected stable device id, got %q then %q", first, second)
	}
	if first == "deepseek_to_api" || strings.Contains(first, "ds2api") {
		t.Fatalf("device id still exposes project fingerprint: %q", first)
	}
	if len(first) != 32 {
		t.Fatalf("expected 16-byte hex device id, got %q", first)
	}
}

func TestLoginDeviceIDSupportsExplicitOverride(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "device-override")

	got := loginDeviceID(config.Account{Email: "user@example.com"})
	if got != "device-override" {
		t.Fatalf("expected override device id, got %q", got)
	}
}
