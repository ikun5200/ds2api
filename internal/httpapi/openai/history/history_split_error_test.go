package history

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	dsclient "ds2api/internal/deepseek/client"
)

func TestMapErrorWithRetryAccountMuted(t *testing.T) {
	until := time.Now().Add(time.Hour)
	err := fmt.Errorf("upload current user input file: %w", &dsclient.RequestFailure{
		Kind:      dsclient.FailureAccountMuted,
		Message:   "user is muted",
		MuteUntil: until,
	})
	status, message, code, retryAfter := MapErrorWithRetry(err)
	if status != http.StatusTooManyRequests || code != "account_muted" {
		t.Fatalf("unexpected mapping: status=%d code=%q message=%q", status, code, message)
	}
	if !strings.Contains(message, "user is muted") {
		t.Fatalf("expected upstream message, got %q", message)
	}
	if retryAfter <= 0 {
		t.Fatalf("expected Retry-After, got %v", retryAfter)
	}
}

func TestMapErrorWithRetryRateLimited(t *testing.T) {
	err := &dsclient.RequestFailure{Kind: dsclient.FailureRateLimited, Message: "rate limit reached"}
	status, _, code, retryAfter := MapErrorWithRetry(err)
	if status != http.StatusTooManyRequests || code != "rate_limited" || retryAfter <= 0 {
		t.Fatalf("unexpected mapping: status=%d code=%q retry=%v", status, code, retryAfter)
	}
}
