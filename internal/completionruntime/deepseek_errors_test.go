package completionruntime

import (
	"net/http"
	"testing"
	"time"

	dsclient "ds2api/internal/deepseek/client"
)

func TestDeepSeekOutputErrorAccountMuted(t *testing.T) {
	until := time.Now().Add(time.Hour)
	out := deepSeekOutputError(&dsclient.RequestFailure{
		Kind:      dsclient.FailureAccountMuted,
		Message:   "user is muted",
		MuteUntil: until,
	})
	if out == nil {
		t.Fatal("expected mapped error")
	}
	if out.Status != http.StatusTooManyRequests || out.Code != "account_muted" {
		t.Fatalf("unexpected output error: %#v", out)
	}
	if out.RetryAfter <= 0 {
		t.Fatalf("expected RetryAfter, got %v", out.RetryAfter)
	}
}

func TestDeepSeekOutputErrorRateLimited(t *testing.T) {
	out := deepSeekOutputError(&dsclient.RequestFailure{
		Kind:       dsclient.FailureRateLimited,
		Message:    "rate limit reached",
		RetryAfter: time.Minute,
	})
	if out == nil || out.Status != http.StatusTooManyRequests || out.Code != "rate_limited" {
		t.Fatalf("unexpected output error: %#v", out)
	}
}

func TestDeepSeekOutputErrorRiskDevice(t *testing.T) {
	out := deepSeekOutputError(&dsclient.RequestFailure{
		Kind:    dsclient.FailureRiskDevice,
		Message: "RISK_DEVICE_DETECTED",
	})
	if out == nil || out.Status != http.StatusForbidden || out.Code != "risk_device" {
		t.Fatalf("unexpected output error: %#v", out)
	}
}
