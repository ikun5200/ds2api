package vercel

import (
	"encoding/json"
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestParseVercelSyncOptionsFallsBackToSavedConfig(t *testing.T) {
	t.Setenv("VERCEL_TOKEN", "")
	t.Setenv("VERCEL_PROJECT_ID", "")
	t.Setenv("VERCEL_TEAM_ID", "")

	opts, err := parseVercelSyncOptions(map[string]any{
		"vercel_token": "__USE_PRECONFIG__",
	}, config.VercelConfig{
		Token:     " saved-token ",
		ProjectID: " saved-project ",
		TeamID:    " saved-team ",
	})
	if err != nil {
		t.Fatalf("parse options error: %v", err)
	}
	if opts.VercelToken != "saved-token" || opts.ProjectID != "saved-project" || opts.TeamID != "saved-team" {
		t.Fatalf("unexpected options: %#v", opts)
	}
	if !opts.UsePreconfig {
		t.Fatal("expected preconfig mode")
	}
}

func TestSaveLocalVercelCredentialsStoresExplicitInput(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"]}`)
	store := config.LoadStore()
	h := &Handler{Store: store}

	saved, err := h.saveLocalVercelCredentials(vercelSyncOptions{
		VercelToken: " token ",
		ProjectID:   " project ",
		TeamID:      " team ",
		SaveCreds:   true,
	})
	if err != nil {
		t.Fatalf("save local credentials error: %v", err)
	}
	if !saved {
		t.Fatal("expected credentials to be saved")
	}
	got := store.Snapshot().Vercel
	if got.Token != "token" || got.ProjectID != "project" || got.TeamID != "team" {
		t.Fatalf("unexpected saved credentials: %#v", got)
	}
}

func TestSaveLocalVercelCredentialsPreservesPreconfiguredTokenAndUpdatesProject(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"],"vercel":{"token":"saved-token","project_id":"old-project","team_id":"old-team"}}`)
	store := config.LoadStore()
	h := &Handler{Store: store}

	saved, err := h.saveLocalVercelCredentials(vercelSyncOptions{
		VercelToken:  "resolved-token",
		ProjectID:    "new-project",
		TeamID:       "new-team",
		SaveCreds:    true,
		UsePreconfig: true,
	})
	if err != nil {
		t.Fatalf("save local credentials error: %v", err)
	}
	if !saved {
		t.Fatal("expected project/team updates to be saved")
	}
	got := store.Snapshot().Vercel
	if got.Token != "saved-token" || got.ProjectID != "new-project" || got.TeamID != "new-team" {
		t.Fatalf("unexpected saved credentials: %#v", got)
	}
}

func TestExportSyncConfigStripsSavedVercelCredentials(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"],"vercel":{"token":"secret-token","project_id":"project","team_id":"team"}}`)
	store := config.LoadStore()
	h := &Handler{Store: store}

	jsonStr, _, err := h.exportSyncConfig(map[string]any{})
	if err != nil {
		t.Fatalf("export sync config error: %v", err)
	}
	if strings.Contains(jsonStr, "secret-token") || strings.Contains(jsonStr, `"vercel"`) {
		t.Fatalf("expected sync export to strip Vercel credentials, got %s", jsonStr)
	}
	var exported config.Config
	if err := json.Unmarshal([]byte(jsonStr), &exported); err != nil {
		t.Fatalf("exported config is invalid JSON: %v", err)
	}
	if len(exported.Keys) != 1 || exported.Keys[0] != "k1" {
		t.Fatalf("unexpected exported config: %#v", exported)
	}
}

func TestVercelCredentialEnvPairsSkipsPreconfiguredTokenOnly(t *testing.T) {
	pairs := vercelCredentialEnvPairs(vercelSyncOptions{
		VercelToken:  "resolved-token",
		ProjectID:    " project ",
		TeamID:       " team ",
		SaveCreds:    true,
		UsePreconfig: true,
	})
	if len(pairs) != 2 {
		t.Fatalf("expected project/team only, got %#v", pairs)
	}
	if pairs[0] != [2]string{"VERCEL_PROJECT_ID", "project"} || pairs[1] != [2]string{"VERCEL_TEAM_ID", "team"} {
		t.Fatalf("unexpected credential env pairs: %#v", pairs)
	}
}

func TestVercelCredentialEnvPairsIncludesExplicitToken(t *testing.T) {
	pairs := vercelCredentialEnvPairs(vercelSyncOptions{
		VercelToken: " token ",
		ProjectID:   " project ",
		SaveCreds:   true,
	})
	if len(pairs) != 2 {
		t.Fatalf("expected token/project pairs, got %#v", pairs)
	}
	if pairs[0] != [2]string{"VERCEL_TOKEN", "token"} || pairs[1] != [2]string{"VERCEL_PROJECT_ID", "project"} {
		t.Fatalf("unexpected credential env pairs: %#v", pairs)
	}
}

func TestSyncHashForCanonicalJSONIsStable(t *testing.T) {
	jsonStr, _, err := encodeVercelSyncConfig(config.Config{Keys: []string{"k1"}})
	if err != nil {
		t.Fatalf("encode sync config error: %v", err)
	}
	got := syncHashForCanonicalJSON(jsonStr)
	if got == "" {
		t.Fatal("expected non-empty canonical hash")
	}
	if again := syncHashForCanonicalJSON(jsonStr); again != got {
		t.Fatalf("canonical hash not stable: %q then %q", got, again)
	}
	if blank := syncHashForCanonicalJSON("  "); blank != "" {
		t.Fatalf("expected blank canonical JSON hash to be empty, got %q", blank)
	}
}

func TestIndexVercelEnvIDs(t *testing.T) {
	envs := []any{
		map[string]any{"key": "DS2API_CONFIG_JSON", "id": "env-config"},
		map[string]any{"key": " VERCEL_PROJECT_ID ", "id": " env-project "},
		map[string]any{"key": "VERCEL_TOKEN"},
	}
	index := indexVercelEnvIDs(envs)
	if index["DS2API_CONFIG_JSON"] != "env-config" || index["VERCEL_PROJECT_ID"] != "env-project" {
		t.Fatalf("unexpected env index: %#v", index)
	}
	if _, ok := index["VERCEL_TOKEN"]; ok {
		t.Fatalf("expected env without id to be skipped, got %#v", index)
	}
}
