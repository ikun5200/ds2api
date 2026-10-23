package responses

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ds2api/internal/auth"
)

func TestWriteAuthDetermineErrorAllMutedSetsRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &auth.AllAccountsMutedError{Until: time.Now().Add(2 * time.Minute), RetryAfter: 2 * time.Minute}
	if !writeAuthDetermineError(rec, err) {
		t.Fatal("expected handled error")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "120" {
		t.Fatalf("expected Retry-After 120, got %q", got)
	}
}
