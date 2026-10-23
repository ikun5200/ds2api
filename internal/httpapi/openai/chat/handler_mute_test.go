package chat

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ds2api/internal/auth"
)

func TestWriteAuthDetermineErrorAllMutedSetsRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &auth.AllAccountsMutedError{Until: time.Now().Add(90 * time.Second), RetryAfter: 90 * time.Second}
	if !writeAuthDetermineError(rec, err) {
		t.Fatal("expected handled error")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("expected Retry-After 90, got %q", got)
	}
}

func TestWriteAuthDetermineErrorPinnedMutedSetsRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	err := &auth.AccountMutedError{Account: "a@example.com", Until: time.Now().Add(time.Minute), RetryAfter: time.Minute}
	if !writeAuthDetermineError(rec, err) {
		t.Fatal("expected handled error")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("expected Retry-After 60, got %q", got)
	}
}
