package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountDeviceIDSurvivesSaveReloadAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("DS2API_CONFIG_PATH", path)
	t.Setenv("DS2API_ENV_WRITEBACK", "1")
	t.Setenv("VERCEL", "")
	t.Setenv("NOW_REGION", "")
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"device@example.com","name":"Device account","password":"password","device_id":" website-issued-device ","token":"ignored-env-token"}]}`)
	store := LoadStore()
	if err := store.UpdateAccountToken("device@example.com", "runtime-token"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(c *Config) error {
		c.Accounts[0].Remark = "updated remark"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	jsonText, encoded, err := store.ExportJSONAndBase64()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != jsonText {
		t.Fatal("base64 export differs from JSON export")
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{[]byte(jsonText), persisted} {
		var cfg Config
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Accounts) != 1 {
			t.Fatal("account missing from serialized config")
		}
		acc := cfg.Accounts[0]
		if acc.DeviceID != "website-issued-device" || acc.Name != "Device account" || acc.Remark != "updated remark" || acc.Password != "password" {
			t.Fatal("serialized account lost device ID or existing fields")
		}
		if acc.Token != "" {
			t.Fatal("runtime token must still be excluded from persistent config")
		}
	}

	t.Setenv("DS2API_CONFIG_JSON", "")
	reloaded := LoadStore()
	acc, ok := reloaded.FindAccount("device@example.com")
	if !ok || acc.DeviceID != "website-issued-device" || acc.Password != "password" || acc.Remark != "updated remark" {
		t.Fatal("reloading saved config lost account device ID or existing fields")
	}
}

func TestAccountDeviceIDConfigValidationAndLegacyCompatibility(t *testing.T) {
	for _, value := range []string{`123`, `true`, `{}`, `[]`} {
		var cfg Config
		if err := json.Unmarshal([]byte(`{"accounts":[{"email":"device@example.com","device_id":`+value+`}]}`), &cfg); err == nil {
			t.Fatalf("expected non-string device_id %s to be rejected", value)
		}
	}
	for _, field := range []string{"", `,"device_id":null`, `,"device_id":""`, `,"device_id":"  "`} {
		var cfg Config
		if err := json.Unmarshal([]byte(`{"accounts":[{"email":"device@example.com"`+field+`}]}`), &cfg); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Accounts[0].DeviceID != "" || strings.Contains(string(raw), `"device_id"`) {
			t.Fatal("an unset device ID should remain optional in old configurations")
		}
	}
}
