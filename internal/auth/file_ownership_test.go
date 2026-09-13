package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func managedFileAuth(t *testing.T, resolver *Resolver) *RequestAuth {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFileAccountSelectionFailureDoesNotReleaseOtherLease(t *testing.T) {
	r := newTestResolver(t)
	first := managedFileAuth(t, r)
	second := managedFileAuth(t, r)
	defer r.Release(second)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := first.SelectFileAccount(ctx, "missing@example.com"); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("expected unavailable owner, got %v", err)
	}
	r.Release(first)
	if first.AccountID != "" || r.Pool.Status()["in_use"] != 1 {
		t.Fatalf("failed selection double-released a lease: auth=%q status=%v", first.AccountID, r.Pool.Status())
	}
}

func TestFileAccountSelectionPreservesUpstreamBinding(t *testing.T) {
	r := newTestResolver(t)
	a := managedFileAuth(t, r)
	defer r.Release(a)
	a.BindUpstreamAccount()
	if err := a.SelectFileAccount(context.Background(), "other@example.com"); !errors.Is(err, ErrFileAccountConflict) {
		t.Fatalf("existing upstream binding was bypassed: %v", err)
	}
	if a.upstreamAccount != a.AccountID || r.Pool.Status()["in_use"] != 1 {
		t.Fatal("rejected selection changed the account binding or lease")
	}
}

func TestFileOwnerCacheExpiresAndIsolatesCallerCredentials(t *testing.T) {
	r := newTestResolver(t)
	a := managedFileAuth(t, r)
	defer r.Release(a)
	a.RememberFileOwner("file-one")
	if owner, ok := a.FileOwner("file-one"); !ok || owner != a.AccountID {
		t.Fatal("successful file ownership was not retained")
	}
	other := *a
	other.CallerID = "another-managed-caller"
	if _, ok := other.FileOwner("file-one"); ok {
		t.Fatal("file ownership crossed caller namespaces")
	}
	direct := *a
	direct.UseConfigToken = false
	if _, ok := direct.FileOwner("file-one"); ok {
		t.Fatal("direct token inherited a managed account owner")
	}
	r.mu.Lock()
	key := a.CallerID + "\x00file-one"
	owner := r.fileOwners[key]
	owner.updatedAt = time.Now().Add(-fileOwnerTTL - time.Second)
	r.fileOwners[key] = owner
	r.mu.Unlock()
	if _, ok := a.FileOwner("file-one"); ok {
		t.Fatal("expired ownership was used for account routing")
	}
}
