package client

import (
	"testing"
	"time"
)

func TestAccountProfileIdentifierAndDisplayName(t *testing.T) {
	emailProfile := AccountProfile{Email: "user@example.com", UserID: "id-1"}
	if got := emailProfile.Identifier(); got != "user@example.com" {
		t.Fatalf("Identifier()=%q", got)
	}
	if got := emailProfile.DisplayName(); got != "user@example.com" {
		t.Fatalf("DisplayName()=%q", got)
	}

	mobileProfile := AccountProfile{Mobile: "13800138000", UserID: "id-2"}
	if got := mobileProfile.Identifier(); got != "13800138000" {
		t.Fatalf("Identifier()=%q", got)
	}

	idProfile := AccountProfile{UserID: "2b94224a-7070-4897-9dfc-9166543c4322"}
	if got := idProfile.Identifier(); got != "token-2b94224a-7070-4897-9dfc-9166543c4322@local" {
		t.Fatalf("Identifier()=%q", got)
	}
	if got := idProfile.DisplayName(); got != "DeepSeek 2b94224a" {
		t.Fatalf("DisplayName()=%q", got)
	}
}

func TestExtractMuteInfoReadsChatMuteState(t *testing.T) {
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	info := extractMuteInfo(map[string]any{
		"data": map[string]any{
			"biz_code": 0,
			"biz_data": map[string]any{
				"chat": map[string]any{
					"is_muted":   1,
					"mute_until": until.Unix(),
				},
			},
		},
	})
	if !info.Muted {
		t.Fatal("expected muted")
	}
	if !info.Until.Equal(until) {
		t.Fatalf("until=%v want=%v", info.Until, until)
	}
}
