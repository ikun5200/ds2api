package client

import (
	"testing"
	"time"
)

func TestExtractMuteInfoUploadResponse(t *testing.T) {
	parsed := map[string]any{
		"code": float64(0),
		"data": map[string]any{
			"biz_code": float64(14),
			"biz_msg":  "user is muted",
			"biz_data": map[string]any{
				"is_muted":   float64(1),
				"mute_until": 1792994003.677,
			},
		},
	}
	info := extractMuteInfo(parsed)
	if !info.Muted {
		t.Fatal("expected muted response")
	}
	if info.Reason != "user is muted" {
		t.Fatalf("unexpected reason: %q", info.Reason)
	}
	if want := time.Unix(1792994003, 0); !info.Until.Equal(want) {
		t.Fatalf("unexpected mute until: %v want %v", info.Until, want)
	}
}

func TestExtractMuteInfoCompletionResponse(t *testing.T) {
	parsed := map[string]any{
		"code": float64(0),
		"data": map[string]any{
			"biz_code": float64(5),
			"biz_msg":  "user is muted",
			"biz_data": map[string]any{
				"is_muted":   float64(1),
				"mute_until": float64(1792994003),
			},
		},
	}
	info := extractMuteInfo(parsed)
	if !info.Muted {
		t.Fatal("expected completion mute response to be detected")
	}
	if info.Until.IsZero() {
		t.Fatal("expected mute_until to be parsed")
	}
}

func TestExtractMuteInfoIgnoresNormalResponse(t *testing.T) {
	parsed := map[string]any{
		"code": float64(0),
		"data": map[string]any{
			"biz_code": float64(0),
			"biz_data": map[string]any{"is_muted": float64(0)},
		},
	}
	if info := extractMuteInfo(parsed); info.Muted {
		t.Fatalf("unexpected mute: %#v", info)
	}
}

func TestRateLimitedAndRiskDeviceClassification(t *testing.T) {
	rate := map[string]any{
		"data": map[string]any{"biz_code": float64(7), "biz_msg": "rate limit reached"},
	}
	if !isRateLimitedResponse(rate) {
		t.Fatal("expected rate limit response")
	}
	risk := map[string]any{
		"data": map[string]any{"biz_code": float64(11), "biz_msg": "RISK_DEVICE_DETECTED"},
	}
	if !isRiskDeviceResponse(risk) {
		t.Fatal("expected risk device response")
	}
}
