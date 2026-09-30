package util

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── EstimateTokens edge cases ───────────────────────────────────────

func TestEstimateTokensEmpty(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("expected 0 for empty string, got %d", got)
	}
}

func TestEstimateTokensShortASCII(t *testing.T) {
	got := EstimateTokens("ab")
	if got != 1 {
		t.Fatalf("expected 1 for 2 ascii chars, got %d", got)
	}
}

func TestEstimateTokensLongASCII(t *testing.T) {
	got := EstimateTokens(strings.Repeat("x", 100))
	if got != 25 {
		t.Fatalf("expected 25 for 100 ascii chars, got %d", got)
	}
}

func TestEstimateTokensChinese(t *testing.T) {
	got := EstimateTokens("你好世界")
	if got < 1 {
		t.Fatalf("expected at least 1 token for Chinese text, got %d", got)
	}
}

func TestEstimateTokensMixed(t *testing.T) {
	got := EstimateTokens("Hello 你好世界")
	if got < 2 {
		t.Fatalf("expected at least 2 tokens for mixed text, got %d", got)
	}
}

func TestEstimateTokensSingleByte(t *testing.T) {
	got := EstimateTokens("x")
	if got != 1 {
		t.Fatalf("expected 1 for single char (minimum), got %d", got)
	}
}

func TestEstimateTokensSingleChinese(t *testing.T) {
	got := EstimateTokens("你")
	if got != 1 {
		t.Fatalf("expected 1 for single Chinese char, got %d", got)
	}
}

// ─── ToBool edge cases ───────────────────────────────────────────────

func TestToBoolTrue(t *testing.T) {
	if !ToBool(true) {
		t.Fatal("expected true")
	}
}

func TestToBoolFalse(t *testing.T) {
	if ToBool(false) {
		t.Fatal("expected false")
	}
}

func TestToBoolNonBool(t *testing.T) {
	if ToBool("true") {
		t.Fatal("expected false for string 'true'")
	}
	if ToBool(1) {
		t.Fatal("expected false for int 1")
	}
	if ToBool(nil) {
		t.Fatal("expected false for nil")
	}
}

// ─── IntFrom edge cases ─────────────────────────────────────────────

func TestIntFromFloat64(t *testing.T) {
	if got := IntFrom(float64(42.5)); got != 42 {
		t.Fatalf("expected 42 for float64(42.5), got %d", got)
	}
}

func TestIntFromInt(t *testing.T) {
	if got := IntFrom(int(42)); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestIntFromInt64(t *testing.T) {
	if got := IntFrom(int64(42)); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
}

func TestIntFromString(t *testing.T) {
	if got := IntFrom("42"); got != 0 {
		t.Fatalf("expected 0 for string, got %d", got)
	}
}

func TestIntFromNil(t *testing.T) {
	if got := IntFrom(nil); got != 0 {
		t.Fatalf("expected 0 for nil, got %d", got)
	}
}

// ─── WriteJSON ───────────────────────────────────────────────────────

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 200, map[string]any{"key": "value"})
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json content type, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["key"] != "value" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestWriteJSONStatusCodes(t *testing.T) {
	for _, code := range []int{200, 201, 400, 404, 500} {
		rec := httptest.NewRecorder()
		WriteJSON(rec, code, map[string]any{"status": code})
		if rec.Code != code {
			t.Fatalf("expected %d, got %d", code, rec.Code)
		}
	}
}

// ─── MessagesPrepare edge cases ──────────────────────────────────────

func TestMessagesPrepareEmpty(t *testing.T) {
	got := MessagesPrepare(nil)
	if got != "" {
		t.Fatalf("expected empty for nil messages, got %q", got)
	}
}

func TestMessagesPrepareMergesConsecutiveSameRole(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hello"},
		{"role": "user", "content": "World"},
	}
	got := MessagesPrepare(messages)
	if !strings.HasPrefix(got, "System instructions:\n") {
		t.Fatalf("expected output integrity guard at the start, got %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
		t.Fatalf("expected both messages, got %q", got)
	}
	// Should be merged into a single user turn with one marker at the start.
	count := strings.Count(got, "User message:\n")
	if count != 1 {
		t.Fatalf("expected one User marker for the merged pair, got %d occurrences", count)
	}
}

func TestMessagesPrepareAssistantMarkers(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hi"},
		{"role": "assistant", "content": "Hello!"},
	}
	got := MessagesPrepare(messages)
	if !strings.Contains(got, "Assistant response:\n") {
		t.Fatalf("expected assistant marker, got %q", got)
	}
	if !strings.Contains(got, "Assistant response:\nHello!\n\n") {
		t.Fatalf("expected assistant response block, got %q", got)
	}
	if strings.Contains(got, "<think>") || strings.Contains(got, "</think>") {
		t.Fatalf("did not expect think tags in prompt, got %q", got)
	}
	if strings.Contains(got, "<system_instructions>") {
		t.Fatalf("did not expect legacy system marker, got %q", got)
	}
}

func TestMessagesPrepareUnknownRole(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hello"},
		{"role": "unknown_role", "content": "Unknown"},
	}
	got := MessagesPrepare(messages)
	if !strings.Contains(got, "Unknown") {
		t.Fatalf("expected unknown role content, got %q", got)
	}
}

func TestMessagesPrepareMarkdownImageReplaced(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Look at this: ![alt](https://example.com/img.png)"},
	}
	got := MessagesPrepare(messages)
	if strings.Contains(got, "![alt]") {
		t.Fatalf("expected markdown image to be replaced, got %q", got)
	}
}

func TestMessagesPrepareNilContent(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": nil},
	}
	got := MessagesPrepare(messages)
	if got != "null" {
		t.Logf("nil content handled as: %q", got)
	}
}
