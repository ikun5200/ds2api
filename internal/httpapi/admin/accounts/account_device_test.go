package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/config"
)

func TestAccountDeviceIDCreateUpdateAndSafeList(t *testing.T) {
	h := newAdminTestHandler(t, `{"accounts":[]}`)
	router := chi.NewRouter()
	RegisterRoutes(router, h)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, adminReq(http.MethodPost, "/accounts", []byte(`{"email":"device@example.com","password":"password","name":"name","remark":"remark","device_id":" website-issued-device "}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("add account: status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct {
		body string
		want string
	}{
		{`{"name":"renamed"}`, "website-issued-device"},
		{`{"device_id":null}`, "website-issued-device"},
		{`{"device_id":" replacement-device "}`, "replacement-device"},
		{`{"device_id":""}`, ""},
	} {
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, adminReq(http.MethodPut, "/accounts/device@example.com", []byte(tc.body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("update account: status=%d body=%s", rec.Code, rec.Body.String())
		}
		acc, ok := h.Store.FindAccount("device@example.com")
		if !ok || acc.DeviceID != tc.want || acc.Password != "password" || acc.Remark != "remark" || acc.Name != "renamed" {
			t.Fatal("updating device ID lost the requested value or other account fields")
		}
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, adminReq(http.MethodGet, "/accounts", nil))
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.Items) != 1 || body.Items[0]["has_device_id"] != (tc.want != "") {
			t.Fatal("account list did not report whether a device ID is configured")
		}
		if _, ok := body.Items[0]["device_id"]; ok || (tc.want != "" && strings.Contains(rec.Body.String(), tc.want)) {
			t.Fatal("account list should not expose the device ID")
		}
	}
}

type deviceAccountDSMock struct {
	testingDSMock
	loginAccount config.Account
}

func (m *deviceAccountDSMock) Login(ctx context.Context, acc config.Account) (string, error) {
	m.loginAccount = acc
	return m.testingDSMock.Login(ctx, acc)
}

func TestAccountTestAcceptsDeviceOverrideWithoutReplacingSavedDevice(t *testing.T) {
	h := newAdminTestHandler(t, `{"accounts":[{"email":"device@example.com","password":"password","device_id":"saved-device"}]}`)
	ds := &deviceAccountDSMock{}
	h.DS = ds
	for _, tc := range []struct {
		field string
		want  string
	}{
		{"", "saved-device"},
		{`,"device_id":null`, "saved-device"},
		{`,"device_id":" test-device "`, "test-device"},
		{`,"device_id":""`, ""},
	} {
		rec := httptest.NewRecorder()
		h.testSingleAccount(rec, adminReq(http.MethodPost, "/accounts/test", []byte(`{"identifier":"device@example.com"`+tc.field+`}`)))
		if rec.Code != http.StatusOK || ds.loginAccount.DeviceID != tc.want || ds.loginAccount.Password != "password" {
			t.Fatalf("test request did not use the requested account device: status=%d", rec.Code)
		}
		acc, ok := h.Store.FindAccount("device@example.com")
		if !ok || acc.DeviceID != "saved-device" {
			t.Fatal("a test-only device override replaced the stored device")
		}
		if strings.Contains(rec.Body.String(), "saved-device") || strings.Contains(rec.Body.String(), "test-device") {
			t.Fatal("account test response exposed a device ID")
		}
	}
}

func TestAccountEndpointsRejectNonStringDeviceIDs(t *testing.T) {
	h := newAdminTestHandler(t, `{"accounts":[{"email":"device@example.com","password":"password","device_id":"saved-device"}]}`)
	ds := &deviceAccountDSMock{}
	h.DS = ds
	router := chi.NewRouter()
	RegisterRoutes(router, h)
	for _, endpoint := range []struct{ method, path, fields string }{
		{http.MethodPost, "/accounts", `"email":"new@example.com"`},
		{http.MethodPut, "/accounts/device@example.com", `"name":"should-not-change"`},
		{http.MethodPost, "/accounts/test", `"identifier":"device@example.com"`},
	} {
		for _, value := range []string{`123`, `false`, `{}`, `[]`} {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, adminReq(endpoint.method, endpoint.path, []byte(`{`+endpoint.fields+`,"device_id":`+value+`}`)))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s with device_id %s: status=%d", endpoint.path, value, rec.Code)
			}
		}
	}
	acc, ok := h.Store.FindAccount("device@example.com")
	if !ok || acc.DeviceID != "saved-device" || acc.Name != "" || len(h.Store.Accounts()) != 1 || ds.loginCalls != 0 {
		t.Fatal("invalid device input mutated accounts or reached the upstream login")
	}
}
