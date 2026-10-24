package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ds2api/internal/account"
	"ds2api/internal/config"
)

func newTokenRefreshResolver(t *testing.T, loginErr error) (*Resolver, *config.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"accounts":[{"email":"tok@example.com","token":"website-token"}],"runtime":{"token_refresh_interval_hours":6}}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", path)
	store := config.LoadStore()
	pool := account.NewPool(store)
	resolver := NewResolver(store, pool, func(context.Context, config.Account) (string, error) {
		return "", loginErr
	})
	return resolver, store
}

// A forced interval refresh must not break requests for token-only accounts:
// the login fallback can never succeed without a password, so the resolver has
// to keep serving with the still-valid website token.
func TestEnsureManagedTokenKeepsTokenWhenForcedRefreshFails(t *testing.T) {
	resolver, store := newTokenRefreshResolver(t, errors.New("login failed"))

	acc, ok := store.FindAccount("tok@example.com")
	if !ok {
		t.Fatal("account missing")
	}
	authCtx := &RequestAuth{UseConfigToken: true, CallerID: "c1", AccountID: acc.Identifier(), Account: acc, TriedAccounts: map[string]bool{}}
	if err := resolver.ensureManagedToken(context.Background(), authCtx); err != nil {
		t.Fatalf("first ensure failed: %v", err)
	}

	resolver.mu.Lock()
	resolver.tokenRefreshedAt[authCtx.AccountID] = time.Now().Add(-6 * time.Hour)
	resolver.mu.Unlock()
	if !resolver.shouldForceRefresh(authCtx.AccountID) {
		t.Fatal("expected forced refresh to trigger")
	}

	if err := resolver.ensureManagedToken(context.Background(), authCtx); err != nil {
		t.Fatalf("ensure must fall back to the website token, got: %v", err)
	}
	if authCtx.DeepSeekToken != "website-token" {
		t.Fatalf("expected website token, got %q", authCtx.DeepSeekToken)
	}

	after, _ := store.FindAccount("tok@example.com")
	if after.Token != "website-token" {
		t.Fatalf("persisted token was modified: %q", after.Token)
	}

	// The refresh mark is untouched, so the next interval retries the refresh
	// instead of hammering password login on every request.
	if resolver.shouldForceRefresh(authCtx.AccountID) {
		t.Fatal("refresh mark should not repeat immediately after a failure")
	}
}
