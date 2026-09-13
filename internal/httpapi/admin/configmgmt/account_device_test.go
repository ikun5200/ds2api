package configmgmt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestConfigMetadataRoundTripPreservesAccountDeviceID(t *testing.T) {
	h := newAdminTestHandler(t, `{"accounts":[{"email":"device@example.com","password":"password","name":"name","remark":"remark","device_id":"saved-device"}]}`)
	rec := httptest.NewRecorder()
	h.getConfig(rec, httptest.NewRequest(http.MethodGet, "/config", nil))
	var safe map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &safe); err != nil {
		t.Fatal(err)
	}
	accounts, _ := safe["accounts"].([]any)
	account, _ := accounts[0].(map[string]any)
	if account["has_device_id"] != true || account["device_id"] != nil || strings.Contains(rec.Body.String(), "saved-device") {
		t.Fatal("safe config should report device presence without exposing the device ID")
	}
	account["name"] = "renamed"
	for _, tc := range []struct {
		value any
		want  string
	}{
		{nil, "saved-device"},
		{" replacement-device ", "replacement-device"},
		{"", ""},
	} {
		if tc.value == nil {
			delete(account, "device_id")
		} else {
			account["device_id"] = tc.value
		}
		body, err := json.Marshal(safe)
		if err != nil {
			t.Fatal(err)
		}
		rec = httptest.NewRecorder()
		h.updateConfig(rec, httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("config update: status=%d body=%s", rec.Code, rec.Body.String())
		}
		acc, ok := h.Store.FindAccount("device@example.com")
		if !ok || acc.DeviceID != tc.want || acc.Password != "password" || acc.Name != "renamed" || acc.Remark != "remark" {
			t.Fatal("safe config roundtrip lost device ID or existing account fields")
		}
	}
}

func TestBatchImportPreservesAccountDeviceID(t *testing.T) {
	h := newAdminTestHandler(t, `{"accounts":[{"email":"existing@example.com","password":"old-password","device_id":"existing-device"}]}`)
	rec := httptest.NewRecorder()
	h.batchImport(rec, httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(`{"accounts":[{"email":"new@example.com","password":"new-password","device_id":" new-device "},{"email":"existing@example.com","device_id":"replacement-device"}]}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("batch import: status=%d body=%s", rec.Code, rec.Body.String())
	}
	existing, ok := h.Store.FindAccount("existing@example.com")
	if !ok || existing.DeviceID != "existing-device" || existing.Password != "old-password" {
		t.Fatal("batch import replaced a duplicate account's existing fields")
	}
	added, ok := h.Store.FindAccount("new@example.com")
	if !ok || added.DeviceID != "new-device" || added.Password != "new-password" {
		t.Fatal("batch import did not preserve the new account's device ID")
	}
}

func TestConfigAccountDeviceValidationIsAtomic(t *testing.T) {
	for _, endpoint := range []string{"config", "import"} {
		for _, value := range []string{`123`, `false`, `{}`, `[]`} {
			h := newAdminTestHandler(t, `{"keys":["existing-key"],"accounts":[{"email":"existing@example.com","device_id":"existing-device"}]}`)
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/"+endpoint, strings.NewReader(`{"keys":["new-key"],"accounts":[{"email":"new@example.com","device_id":`+value+`}]}`))
			if endpoint == "config" {
				h.updateConfig(rec, req)
			} else {
				h.batchImport(rec, req)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s with device_id %s: status=%d", endpoint, value, rec.Code)
			}
			snap := h.Store.Snapshot()
			if len(snap.Keys) != 1 || snap.Keys[0] != "existing-key" || len(snap.Accounts) != 1 || snap.Accounts[0].DeviceID != "existing-device" {
				t.Fatal("invalid device ID partially mutated configuration")
			}
		}
	}
}

func TestConfigImportExportRetainsDeviceIDAndStripsRuntimeToken(t *testing.T) {
	for _, mode := range []string{"merge", "replace"} {
		h := newAdminTestHandler(t, `{"accounts":[]}`)
		rec := httptest.NewRecorder()
		h.configImport(rec, httptest.NewRequest(http.MethodPost, "/config/import?mode="+mode, strings.NewReader(`{"config":{"accounts":[{"email":"device@example.com","password":"password","device_id":" imported-device ","token":"ignored-token"}]}}`)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s import: status=%d body=%s", mode, rec.Code, rec.Body.String())
		}
		jsonText, _, err := h.Store.ExportJSONAndBase64()
		if err != nil {
			t.Fatal(err)
		}
		var exported config.Config
		if err := json.Unmarshal([]byte(jsonText), &exported); err != nil {
			t.Fatal(err)
		}
		if len(exported.Accounts) != 1 || exported.Accounts[0].DeviceID != "imported-device" || exported.Accounts[0].Password != "password" || exported.Accounts[0].Token != "" {
			t.Fatal("config import/export did not preserve the persistent device ID separately from the runtime token")
		}
	}
}
