package account

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ds2api/internal/config"
)

func newMuteTestPool(t *testing.T, healthPath string) *Pool {
	t.Helper()
	t.Setenv("DS2API_ACCOUNT_HEALTH_PATH", healthPath)
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["k1"],
		"accounts":[
			{"email":"acc1@example.com","token":"token1"},
			{"email":"acc2@example.com","token":"token2"}
		]
	}`)
	return NewPool(config.LoadStore())
}

func TestPoolMuteSkipsAccountAndReportsUnmuteTime(t *testing.T) {
	pool := newPoolForTest(t, "2")
	until := time.Now().Add(time.Hour).Truncate(time.Second)
	pool.MarkMuted("acc1@example.com", until, "user is muted")

	if _, ok := pool.Acquire("acc1@example.com", nil); ok {
		t.Fatal("muted pinned account should not be acquired")
	}
	acc, ok := pool.Acquire("", nil)
	if !ok || acc.Identifier() != "acc2@example.com" {
		t.Fatalf("expected non-muted account, got %#v ok=%v", acc, ok)
	}
	pool.Release(acc.Identifier())

	status := pool.Status()
	if got, _ := status["muted"].(int); got != 1 {
		t.Fatalf("expected one muted account, got %#v", status["muted"])
	}
	if got, _ := status["earliest_unmute_at"].(int64); got != until.Unix() {
		t.Fatalf("expected earliest unmute %d, got %#v", until.Unix(), status["earliest_unmute_at"])
	}

	pool.ClearMute("acc1@example.com")
	if _, ok := pool.Acquire("acc1@example.com", nil); !ok {
		t.Fatal("cleared account should be acquirable")
	}
}

func TestPoolAllMuted(t *testing.T) {
	pool := newPoolForTest(t, "2")
	until := time.Now().Add(30 * time.Minute)
	pool.MarkMuted("acc1@example.com", until, "user is muted")
	pool.MarkMuted("acc2@example.com", until.Add(time.Minute), "user is muted")
	if !pool.AllMuted() {
		t.Fatal("expected all accounts muted")
	}
	if got, ok := pool.EarliestMutedUntil(); !ok || !got.Equal(until) {
		t.Fatalf("unexpected earliest unmute: %v ok=%v", got, ok)
	}
}

func TestPoolMuteStatePersistsAcrossRestart(t *testing.T) {
	healthPath := filepath.Join(t.TempDir(), "account_health.json")
	pool := newMuteTestPool(t, healthPath)
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	pool.MarkMuted("acc1@example.com", until, "user is muted")

	reloaded := newMuteTestPool(t, healthPath)
	state, ok := reloaded.MuteState("acc1@example.com")
	if !ok {
		t.Fatal("expected mute state to be reloaded")
	}
	if !state.MutedUntil.Equal(until) || state.Reason != "user is muted" {
		t.Fatalf("unexpected reloaded state: %#v", state)
	}
}

func TestPoolAcquireWaitAllMutedReturnsImmediately(t *testing.T) {
	pool := newPoolForTest(t, "2")
	until := time.Now().Add(30 * time.Minute)
	pool.MarkMuted("acc1@example.com", until, "user is muted")
	pool.MarkMuted("acc2@example.com", until, "user is muted")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, ok := pool.AcquireWait(ctx, "", nil); ok {
		t.Fatal("expected no account while all are muted")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("all-muted acquire should return immediately, took %v", elapsed)
	}
}

func TestPoolMutedAccountsDueAndProbeResult(t *testing.T) {
	pool := newPoolForTest(t, "2")
	past := time.Now().Add(-10 * time.Minute)
	pool.MarkMuted("acc1@example.com", past, "user is muted")

	due := pool.MutedAccountsDue(time.Now())
	if len(due) != 1 || due[0] != "acc1@example.com" {
		t.Fatalf("expected account to be due for probe, got %#v", due)
	}

	pool.MarkProbeResult("acc1@example.com", false, time.Time{}, "", nil)
	if _, ok := pool.MuteState("acc1@example.com"); ok {
		t.Fatal("successful probe should clear mute state")
	}
}

func TestPoolMarkProbeErrorKeepsMuteAndBacksOff(t *testing.T) {
	pool := newPoolForTest(t, "2")
	past := time.Now().Add(-10 * time.Minute)
	pool.MarkMuted("acc1@example.com", past, "user is muted")

	pool.MarkProbeResult("acc1@example.com", true, time.Time{}, "", assertError("probe failed"))
	state, ok := pool.MuteState("acc1@example.com")
	if !ok {
		t.Fatal("expected account to remain muted after probe error")
	}
	if state.LastError != "probe failed" {
		t.Fatalf("unexpected last error: %#v", state)
	}
	if !state.NextProbeAt.After(time.Now()) {
		t.Fatalf("expected probe backoff, got %v", state.NextProbeAt)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }
