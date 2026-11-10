package shared

import "testing"

func TestClonePayloadForEmptyOutputRetryDisablesThinking(t *testing.T) {
	payload := map[string]any{
		"prompt":           "User message:\nhello",
		"thinking_enabled": true,
		"search_enabled":   false,
	}
	retry := ClonePayloadForEmptyOutputRetry(payload, 42)

	if retry["thinking_enabled"] != false {
		t.Fatalf("expected thinking disabled on retry, got %#v", retry["thinking_enabled"])
	}
	if payload["thinking_enabled"] != true {
		t.Fatalf("original payload must stay untouched, got %#v", payload["thinking_enabled"])
	}
	if retry["parent_message_id"] != 42 {
		t.Fatalf("parent_message_id=%#v", retry["parent_message_id"])
	}
	prompt, _ := retry["prompt"].(string)
	if prompt == "" || prompt == payload["prompt"] {
		t.Fatalf("expected retry prompt suffix, got %q", prompt)
	}
}
