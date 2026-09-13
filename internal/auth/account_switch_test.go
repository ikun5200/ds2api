package auth

import (
	"context"
	"errors"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/config"
)

func TestFailedFreshAccountSwitchPreservesConcurrentLeases(t *testing.T) {
	r := newTestResolver(t)
	first := managedFileAuth(t, r)
	second := managedFileAuth(t, r)
	defer r.Release(second)
	first.BindUpstreamAccount()
	originalID, originalToken := first.AccountID, first.DeepSeekToken
	if first.SwitchAccountForFreshCompletion(context.Background()) {
		t.Fatal("single account cannot switch")
	}
	if first.AccountID != originalID || first.DeepSeekToken != originalToken || first.upstreamAccount != originalID {
		t.Fatal("failed switch must retain its original resource owner and credential")
	}
	if got := r.Pool.Status()["in_use"]; got != 2 {
		t.Fatalf("failed switch gave up its lease while retaining its credential: %v", got)
	}
	r.Release(first)
	if got := r.Pool.Status()["in_use"]; got != 1 {
		t.Fatalf("failed switch released another request's lease: %v", got)
	}
}

func newSwitchResolver(t *testing.T, login LoginFunc) *Resolver {
	t.Helper()
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["managed-key"],"accounts":[{"email":"a@example.com","password":"p"},{"email":"b@example.com","password":"p"}],"runtime":{"account_max_inflight":2,"global_max_inflight":4}}`)
	t.Setenv("DS2API_ENV_WRITEBACK", "0")
	store := config.LoadStore()
	return NewResolver(store, account.NewPool(store), login)
}

func TestFreshAccountSwitchWorksAtGlobalConcurrencyLimitOne(t *testing.T) {
	r := newSwitchResolver(t, func(_ context.Context, acc config.Account) (string, error) {
		return "token-" + acc.Identifier(), nil
	})
	r.Pool.ApplyRuntimeLimits(1, 0, 1)
	a := managedFileAuth(t, r)
	defer r.Release(a)
	a.BindUpstreamAccount()
	if !a.SwitchAccountForFreshCompletion(context.Background()) {
		t.Fatal("fresh completion must transfer its sole global slot to the next account")
	}
	if a.AccountID != "b@example.com" || a.DeepSeekToken != "token-b@example.com" || a.upstreamAccount != "" {
		t.Fatalf("fresh switch did not select a new credential without the old resource binding: account=%s binding=%s", a.AccountID, a.upstreamAccount)
	}
	if got := r.Pool.Status()["in_use"]; got != 1 {
		t.Fatalf("switch must retain exactly one global lease: %v", got)
	}
	if extra, ok := r.Pool.Acquire("", nil); ok {
		r.Pool.Release(extra.Identifier())
		t.Fatal("switch left capacity for a request above the global limit")
	}
}

func TestFailedCandidateLoginDoesNotReuseUnleasedCredential(t *testing.T) {
	for _, fillOldSlot := range []bool{false, true} {
		t.Run(map[bool]string{false: "restore original lease", true: "original account now full"}[fillOldSlot], func(t *testing.T) {
			var r *Resolver
			var extraLease string
			r = newSwitchResolver(t, func(_ context.Context, acc config.Account) (string, error) {
				if acc.Identifier() == "b@example.com" {
					if fillOldSlot {
						extra, ok := r.Pool.Acquire("a@example.com", nil)
						if !ok {
							t.Fatal("test must occupy the original slot while candidate login runs")
						}
						extraLease = extra.Identifier()
					}
					return "", errors.New("candidate login failed")
				}
				return "token-" + acc.Identifier(), nil
			})
			a := managedFileAuth(t, r)
			other, ok := r.Pool.Acquire(a.AccountID, nil)
			if !ok {
				t.Fatal("test must acquire a concurrent lease on the original account")
			}
			defer r.Pool.Release(other.Identifier())
			defer func() { r.Pool.Release(extraLease) }()
			a.BindUpstreamAccount()
			if a.SwitchAccountForFreshCompletion(context.Background()) {
				t.Fatal("failed candidate login must not be reported as a successful switch")
			}
			if fillOldSlot {
				if a.AccountID != "" || a.DeepSeekToken != "" || a.upstreamAccount != "" {
					t.Fatalf("failed switch retained an account credential without its lease: account=%s binding=%s", a.AccountID, a.upstreamAccount)
				}
			} else if a.AccountID != "a@example.com" || a.DeepSeekToken != "token-a@example.com" || a.upstreamAccount != a.AccountID {
				t.Fatalf("failed switch did not restore its original account: account=%s binding=%s", a.AccountID, a.upstreamAccount)
			}
			r.Release(a)
			wantLeases := 1
			if fillOldSlot {
				wantLeases = 2
			}
			if got := r.Pool.Status()["in_use"]; got != wantLeases {
				t.Fatalf("failed candidate login changed concurrent leases: got=%v want=%d", got, wantLeases)
			}
		})
	}
}
