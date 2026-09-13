package account

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"ds2api/internal/config"
)

func newResetPool(t *testing.T, maxPer, global int) *Pool {
	t.Helper()
	t.Setenv("DS2API_ENV_WRITEBACK", "0")
	t.Setenv("DS2API_CONFIG_JSON", fmt.Sprintf(`{"keys":["key"],"accounts":[{"email":"acc1@example.com","token":"t1"},{"email":"acc2@example.com","token":"t2"}],"runtime":{"account_max_inflight":%d,"global_max_inflight":%d,"account_max_queue":4}}`, maxPer, global))
	return NewPool(config.LoadStore())
}

func TestPoolResetPreservesConcurrentAccountLeases(t *testing.T) {
	pool := newResetPool(t, 2, 4)
	first, ok := pool.Acquire("acc1@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire first account lease")
	}
	if err := pool.store.Update(func(c *config.Config) error {
		c.Accounts[0].DeviceID = "updated-device"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	pool.Reset()
	second, ok := pool.Acquire(first.Identifier(), nil)
	if !ok || second.DeviceID != "updated-device" {
		t.Fatal("second slot must use updated account metadata")
	}
	defer pool.Release(second.Identifier())
	if extra, ok := pool.Acquire(first.Identifier(), nil); ok {
		pool.Release(extra.Identifier())
		t.Fatal("reset forgot an active lease and exceeded the per-account limit")
	}
	pool.Release(first.Identifier())
	if got := pool.Status()["in_use"]; got != 1 {
		t.Fatalf("pre-reset request released another request's lease: %v", got)
	}
}

func TestPoolResetRetainsRemovedAndDisabledAccountLeases(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprintf("removed=%v", remove), func(t *testing.T) {
			pool := newResetPool(t, 1, 1)
			first, ok := pool.Acquire("acc1@example.com", nil)
			if !ok {
				t.Fatal("failed to acquire old account")
			}
			if err := pool.store.Update(func(c *config.Config) error {
				if remove {
					c.Accounts = c.Accounts[1:]
				} else {
					c.Accounts[0].Disabled = true
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			pool.Reset()
			if got := pool.Status()["in_use"]; got != 1 {
				t.Fatalf("removed or disabled account's active lease disappeared: %v", got)
			}
			if extra, ok := pool.Acquire(first.Identifier(), nil); ok {
				pool.Release(extra.Identifier())
				t.Fatal("removed or disabled account accepted a new request")
			}
			if extra, ok := pool.Acquire("acc2@example.com", nil); ok {
				pool.Release(extra.Identifier())
				t.Fatal("reset exceeded the global limit while an old request was active")
			}
			pool.Release(first.Identifier())
			second, ok := pool.Acquire("acc2@example.com", nil)
			if !ok {
				t.Fatal("returning the removed account's lease did not free the global slot")
			}
			pool.Release(second.Identifier())
			if got := pool.Status()["in_use"]; got != 0 {
				t.Fatalf("removed account left an unreturnable lease: %v", got)
			}
		})
	}
}

func TestPoolResetKeepsWaiterBehindHeldGlobalSlot(t *testing.T) {
	pool := newResetPool(t, 1, 1)
	first, ok := pool.Acquire("acc1@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire global slot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := make(chan bool, 1)
	go func() {
		_, ok := pool.AcquireWait(ctx, "acc2@example.com", nil)
		result <- ok
	}()
	waitForWaitingCount(t, pool, 1)
	pool.Reset()
	waitForWaitingCount(t, pool, 1)
	pool.Release(first.Identifier())
	if !<-result {
		t.Fatal("waiter must acquire after the original request returns its slot")
	}
	if got := pool.Status()["in_use"]; got != 1 {
		t.Fatalf("waiter lease was lost across reset: %v", got)
	}
	pool.Release("acc2@example.com")
}

func TestPoolResetAllowsSwapFromRemovedAccount(t *testing.T) {
	for _, destinationFull := range []bool{false, true} {
		t.Run(fmt.Sprintf("destination_full=%v", destinationFull), func(t *testing.T) {
			pool := newResetPool(t, 1, 2)
			first, ok := pool.Acquire("acc1@example.com", nil)
			if !ok {
				t.Fatal("failed to acquire old account")
			}
			if destinationFull {
				if _, ok := pool.Acquire("acc2@example.com", nil); !ok {
					t.Fatal("failed to occupy destination")
				}
				defer pool.Release("acc2@example.com")
			}
			if err := pool.store.Update(func(c *config.Config) error {
				c.Accounts = c.Accounts[1:]
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			pool.Reset()
			next, swapped := pool.Swap(first.Identifier(), "acc2@example.com", nil)
			if swapped == destinationFull {
				t.Fatalf("removed account lease transfer: swapped=%v destination_full=%v", swapped, destinationFull)
			}
			if swapped {
				pool.Release(next.Identifier())
				if got := pool.Status()["in_use"]; got != 0 {
					t.Fatalf("successful transfer retained an old lease: %v", got)
				}
			} else {
				if got := pool.Status()["in_use"]; got != 2 {
					t.Fatalf("failed transfer lost the removed account's lease: %v", got)
				}
				pool.Release(first.Identifier())
				if got := pool.Status()["in_use"]; got != 1 {
					t.Fatalf("failed transfer released the destination's lease: %v", got)
				}
			}
		})
	}
}

func TestAutomaticSelectionSkipsNewlyDisabledAccountBeforeReset(t *testing.T) {
	for _, swap := range []bool{false, true} {
		t.Run(fmt.Sprintf("swap=%v", swap), func(t *testing.T) {
			pool := newResetPool(t, 1, 2)
			if swap {
				if _, ok := pool.Acquire("acc2@example.com", nil); !ok {
					t.Fatal("failed to acquire source account")
				}
				defer pool.Release("acc2@example.com")
			}
			if err := pool.store.Update(func(c *config.Config) error {
				c.Accounts[0].Disabled = true
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			// An admin update writes the Store before refreshing the queue.
			if swap {
				if next, ok := pool.Swap("acc2@example.com", "", map[string]bool{"acc2@example.com": true}); ok {
					pool.Release(next.Identifier())
					t.Fatal("automatic switch selected an account already disabled in the Store")
				}
			} else {
				next, ok := pool.Acquire("", nil)
				if !ok || next.Identifier() != "acc2@example.com" {
					t.Fatal("automatic acquisition did not skip the newly disabled account")
				}
				pool.Release(next.Identifier())
			}
		})
	}
}

func TestPoolResetConcurrentOperationsKeepActiveLeases(t *testing.T) {
	pool := newResetPool(t, 2, 4)
	stable, ok := pool.Acquire("acc1@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire stable request lease")
	}
	switching, ok := pool.Acquire("acc2@example.com", nil)
	if !ok {
		t.Fatal("failed to acquire switching request lease")
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(4)
	go func() {
		defer workers.Done()
		<-start
		for range 32 {
			pool.Reset()
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for range 128 {
			pool.ApplyRuntimeLimits(2, 4, 4)
			pool.Status()
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for range 128 {
			if acc, ok := pool.Acquire("acc2@example.com", nil); ok {
				pool.Release(acc.Identifier())
			}
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		current := switching.Identifier()
		for range 128 {
			if next, ok := pool.Swap(current, "", map[string]bool{current: true}); ok {
				current = next.Identifier()
			}
		}
		pool.Release(current)
	}()
	close(start)
	workers.Wait()
	if got := pool.Status()["in_use"]; got != 1 {
		t.Fatalf("concurrent reset changed the long-running request's lease: %v", got)
	}
	pool.Release(stable.Identifier())
	if got := pool.Status()["in_use"]; got != 0 {
		t.Fatalf("concurrent operations leaked a lease: %v", got)
	}
}
