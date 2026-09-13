package vercel

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"ds2api/internal/config"
)

func TestVercelSyncPreservesAccountDeviceID(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"device@example.com","password":"password","device_id":"saved-device"}]}`)
	store := config.LoadStore()
	if err := store.UpdateAccountToken("device@example.com", "runtime-token"); err != nil {
		t.Fatal(err)
	}
	h := &Handler{Store: store}
	for _, req := range []map[string]any{
		{},
		{"config_override": map[string]any{"accounts": []any{map[string]any{
			"email": "device@example.com", "password": "password", "device_id": "saved-device", "token": "override-token",
		}}}},
	} {
		jsonText, encoded, err := h.exportSyncConfig(req)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || string(decoded) != jsonText {
			t.Fatal("Vercel base64 config differs from the JSON export")
		}
		var exported config.Config
		if err := json.Unmarshal(decoded, &exported); err != nil {
			t.Fatal(err)
		}
		if len(exported.Accounts) != 1 || exported.Accounts[0].DeviceID != "saved-device" || exported.Accounts[0].Token != "" || exported.Accounts[0].Password != "password" {
			t.Fatal("Vercel sync did not preserve device credentials independently of runtime tokens")
		}
	}
	before := h.computeSyncHash()
	if err := store.Update(func(c *config.Config) error {
		c.Accounts[0].DeviceID = "replacement-device"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if after := h.computeSyncHash(); after == before {
		t.Fatal("a changed device ID must require Vercel config synchronization")
	}
}
