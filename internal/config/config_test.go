package config

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

func TestAccountIdentifierRequiresEmailOrMobile(t *testing.T) {
	acc := Account{Token: "example-token-value"}
	id := acc.Identifier()
	if id != "" {
		t.Fatalf("expected empty identifier when only token is present, got %q", id)
	}
}

func TestLoadStoreClearsTokensFromConfigInput(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["k1"],
		"accounts":[{"email":"u@example.com","password":"p","token":"token-only-account"}]
	}`)

	store := LoadStore()
	accounts := store.Accounts()
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Token != "" {
		t.Fatalf("expected token to be cleared after loading, got %q", accounts[0].Token)
	}
}

func TestLoadStoreDropsLegacyTokenOnlyAccounts(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"accounts":[
			{"token":"legacy-token-only"},
			{"email":"u@example.com","password":"p","token":"runtime-token"}
		]
	}`)

	store := LoadStore()
	accounts := store.Accounts()
	if len(accounts) != 1 {
		t.Fatalf("expected token-only account to be dropped, got %d accounts", len(accounts))
	}
	if accounts[0].Identifier() != "u@example.com" {
		t.Fatalf("unexpected remaining account: %#v", accounts[0])
	}
	if accounts[0].Token != "" {
		t.Fatalf("expected persisted token to be cleared, got %q", accounts[0].Token)
	}
}

func TestLoadStorePreservesFileBackedTokensForRuntime(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	defer tmp.Close()

	if _, err := tmp.WriteString(`{
		"accounts":[{"email":"u@example.com","password":"p","token":"persisted-token"}]
	}`); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", tmp.Name())

	store := LoadStore()
	accounts := store.Accounts()
	if len(accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accounts))
	}
	if accounts[0].Token != "persisted-token" {
		t.Fatalf("expected file-backed token preserved for runtime use, got %q", accounts[0].Token)
	}
}

func TestRuntimeTokenRefreshIntervalHoursDefaultsToSix(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["k1"],
		"accounts":[{"email":"u@example.com","password":"p"}]
	}`)

	store := LoadStore()
	if got := store.RuntimeTokenRefreshIntervalHours(); got != 6 {
		t.Fatalf("expected default refresh interval 6, got %d", got)
	}
}

func TestRuntimeTokenRefreshIntervalHoursUsesConfigValue(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["k1"],
		"accounts":[{"email":"u@example.com","password":"p"}],
		"runtime":{"token_refresh_interval_hours":9}
	}`)

	store := LoadStore()
	if got := store.RuntimeTokenRefreshIntervalHours(); got != 9 {
		t.Fatalf("expected configured refresh interval 9, got %d", got)
	}
}

func TestStoreUpdateAccountTokenKeepsIdentifierResolvable(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"accounts":[{"email":"user@example.com","password":"p"}]
	}`)

	store := LoadStore()
	before := store.Accounts()
	if len(before) != 1 {
		t.Fatalf("expected 1 account, got %d", len(before))
	}
	oldID := before[0].Identifier()
	if err := store.UpdateAccountToken(oldID, "new-token"); err != nil {
		t.Fatalf("update token failed: %v", err)
	}

	if got, ok := store.FindAccount(oldID); !ok || got.Token != "new-token" {
		t.Fatalf("expected find by stable account identifier")
	}
}

func TestLoadStoreRejectsInvalidFieldType(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":"not-array","accounts":[]}`)
	store := LoadStore()
	if len(store.Keys()) != 0 || len(store.Accounts()) != 0 {
		t.Fatalf("expected empty store when config type is invalid")
	}
}

func TestParseConfigStringSupportsQuotedBase64Prefix(t *testing.T) {
	rawJSON := `{"keys":["k1"],"accounts":[{"email":"u@example.com","password":"p"}]}`
	b64 := base64.StdEncoding.EncodeToString([]byte(rawJSON))
	cfg, err := parseConfigString(`"base64:` + b64 + `"`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0] != "k1" {
		t.Fatalf("unexpected keys: %#v", cfg.Keys)
	}
}

func TestParseConfigStringSupportsRawURLBase64(t *testing.T) {
	rawJSON := `{"keys":["k-url"],"accounts":[]}`
	b64 := base64.RawURLEncoding.EncodeToString([]byte(rawJSON))
	cfg, err := parseConfigString(b64)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0] != "k-url" {
		t.Fatalf("unexpected keys: %#v", cfg.Keys)
	}
}

func TestLoadConfigOnVercelWithoutConfigFileFallsBackToMemory(t *testing.T) {
	t.Setenv("VERCEL", "1")
	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", "testdata/does-not-exist.json")

	cfg, fromEnv, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fromEnv {
		t.Fatalf("expected fromEnv=true for vercel fallback")
	}
	if len(cfg.Keys) != 0 || len(cfg.Accounts) != 0 {
		t.Fatalf("expected empty bootstrap config, got keys=%d accounts=%d", len(cfg.Keys), len(cfg.Accounts))
	}
}

func TestAccountTestStatusIsRuntimeOnlyAndNotPersisted(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	defer tmp.Close()
	if _, err := tmp.WriteString(`{
		"accounts":[{"email":"u@example.com","password":"p","test_status":"ok"}]
	}`); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", tmp.Name())

	store := LoadStore()
	if got, ok := store.AccountTestStatus("u@example.com"); ok || got != "" {
		t.Fatalf("expected no runtime status loaded from config, got %q", got)
	}
	if err := store.UpdateAccountTestStatus("u@example.com", "ok"); err != nil {
		t.Fatalf("update test status: %v", err)
	}
	if got, ok := store.AccountTestStatus("u@example.com"); !ok || got != "ok" {
		t.Fatalf("expected runtime status to be available, got %q (ok=%v)", got, ok)
	}

	content, err := os.ReadFile(tmp.Name())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(content), "test_status") {
		t.Fatalf("expected test_status to stay out of persisted config, got: %s", content)
	}
}
