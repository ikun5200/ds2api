package account

import (
	"context"
	"testing"
	"time"

	"ds2api/internal/config"
)

func TestPoolSwapPreservesLeaseWhenDestinationIsFull(t *testing.T) {
	pool := newPoolForTest(t, "1")
	first, ok := pool.Acquire("acc1@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire source account")
	}
	second, ok := pool.Acquire("acc2@example.com", nil)
	if !ok {
		t.Fatal("failed to fill destination account")
	}
	defer pool.Release(second.Identifier())
	if _, ok := pool.Swap(first.Identifier(), second.Identifier(), nil); ok {
		t.Fatal("swap exceeded the destination's per-account limit")
	}
	if got := pool.Status()["in_use"]; got != 2 {
		t.Fatalf("failed swap lost its active lease: %v", got)
	}
	pool.Release(first.Identifier())
	if got := pool.Status()["in_use"]; got != 1 {
		t.Fatalf("releasing a failed swap changed the destination lease: %v", got)
	}
}

func TestPoolSwapKeepsQueuedRequestBehindGlobalLimit(t *testing.T) {
	pool := newPoolForTest(t, "1")
	pool.ApplyRuntimeLimits(1, 1, 1)
	first, ok := pool.Acquire("acc1@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire global slot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	type acquired struct {
		account config.Account
		ok      bool
	}
	result := make(chan acquired, 1)
	go func() {
		acc, ok := pool.AcquireWait(ctx, "acc1@example.com", nil)
		result <- acquired{account: acc, ok: ok}
	}()
	waitForWaitingCount(t, pool, 1)
	next, ok := pool.Swap(first.Identifier(), "acc2@example.com", nil)
	if !ok {
		t.Fatal("queued request stole the global slot during account switch")
	}
	defer pool.Release(next.Identifier())
	waitForWaitingCount(t, pool, 1)
	cancel()
	queued := <-result
	if queued.ok {
		pool.Release(queued.account.Identifier())
		t.Fatal("account switch allowed the queued request above the global limit")
	}
	if got := pool.Status()["in_use"]; got != 1 {
		t.Fatalf("account switch must retain one global lease: %v", got)
	}
}
