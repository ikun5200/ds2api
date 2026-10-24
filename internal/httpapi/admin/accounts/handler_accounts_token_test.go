package accounts

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/account"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
)

type tokenProfileDSMock struct {
	testingDSMock
	profile *dsclient.AccountProfile
	err     error
}

func (m *tokenProfileDSMock) GetCurrentUserForToken(_ context.Context, _ string) (*dsclient.AccountProfile, error) {
	return m.profile, m.err
}

func newTokenHandler(t *testing.T, rawConfig string, ds *tokenProfileDSMock) (*Handler, http.Handler) {
	t.Helper()
	t.Setenv("DS2API_CONFIG_JSON", rawConfig)
	store := config.LoadStore()
	h := &Handler{Store: store, Pool: account.NewPool(store), DS: ds}
	r := chi.NewRouter()
	r.Post("/accounts/token-info", h.tokenInfo)
	r.Post("/accounts", h.addAccount)
	r.Put("/accounts/{identifier}", h.updateAccount)
	return h, r
}

func TestTokenInfoReturnsAccountProfile(t *testing.T) {
	until := time.Now().Add(time.Hour).Truncate(time.Second)
	ds := &tokenProfileDSMock{profile: &dsclient.AccountProfile{
		UserID:    "user-1",
		Token:     "tok",
		Email:     "masked@example.com",
		Name:      "Masked Account",
		Muted:     true,
		MuteUntil: until,
	}}
	_, router := newTokenHandler(t, `{"accounts":[]}`, ds)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/accounts/token-info", bytes.NewReader([]byte(`{"token":"tok"}`))))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["email"] != "masked@example.com" || payload["name"] != "Masked Account" {
		t.Fatalf("unexpected profile: %#v", payload)
	}
	if muted, _ := payload["muted"].(bool); !muted {
		t.Fatalf("expected muted=true: %#v", payload)
	}
	if payload["suggested_identifier"] != "masked@example.com" {
		t.Fatalf("unexpected identifier: %#v", payload)
	}
}

func TestAddAccountWithTokenOnlyFillsProfileFields(t *testing.T) {
	ds := &tokenProfileDSMock{profile: &dsclient.AccountProfile{
		UserID: "user-1",
		Token:  "tok",
		Email:  "masked@example.com",
		Name:   "Masked Account",
	}}
	h, router := newTokenHandler(t, `{"accounts":[]}`, ds)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewReader([]byte(`{"token":"tok"}`))))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	acc, ok := h.Store.FindAccount("masked@example.com")
	if !ok {
		t.Fatal("expected account created from token profile")
	}
	if acc.Token != "tok" || acc.Name != "Masked Account" || acc.Email != "masked@example.com" {
		t.Fatalf("unexpected account: %#v", acc)
	}
}

func TestUpdateAccountTokenPersists(t *testing.T) {
	ds := &tokenProfileDSMock{profile: &dsclient.AccountProfile{
		UserID: "user-1",
		Token:  "new-token",
		Email:  "token@example.com",
	}}
	h, router := newTokenHandler(t, `{"accounts":[{"email":"token@example.com","password":"pwd"}]}`, ds)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/accounts/token@example.com", bytes.NewReader([]byte(`{"token":"new-token"}`))))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	acc, ok := h.Store.FindAccount("token@example.com")
	if !ok || acc.Token != "new-token" {
		t.Fatalf("token not persisted: %#v", acc)
	}
}

func TestTestAccountUsesConfiguredTokenWithoutLogin(t *testing.T) {
	ds := &tokenProfileDSMock{profile: &dsclient.AccountProfile{
		UserID: "user-1",
		Token:  "configured-token",
		Email:  "token@example.com",
	}}
	h, _ := newTokenHandler(t, `{"accounts":[{"email":"token@example.com","token":"configured-token"}]}`, ds)
	if err := h.Store.UpdateAccountToken("token@example.com", "configured-token"); err != nil {
		t.Fatal(err)
	}
	acc, ok := h.Store.FindAccount("token@example.com")
	if !ok {
		t.Fatal("expected account")
	}

	result := h.testAccount(context.Background(), acc, "deepseek-flash", "")
	if ok, _ := result["success"].(bool); !ok {
		t.Fatalf("expected success: %#v", result)
	}
	if ds.loginCalls != 0 {
		t.Fatalf("expected no login call, got %d", ds.loginCalls)
	}
	if result["token_source"] != "token" {
		t.Fatalf("unexpected token source: %#v", result)
	}
	if msg, _ := result["message"].(string); !strings.Contains(msg, "Token 验证成功") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestTestAccountRejectsMutedToken(t *testing.T) {
	until := time.Now().Add(time.Hour).Truncate(time.Second)
	ds := &tokenProfileDSMock{profile: &dsclient.AccountProfile{
		UserID:    "user-1",
		Token:     "configured-token",
		Email:     "token@example.com",
		Muted:     true,
		MuteUntil: until,
	}}
	h, _ := newTokenHandler(t, `{"accounts":[{"email":"token@example.com","token":"configured-token"}]}`, ds)
	if err := h.Store.UpdateAccountToken("token@example.com", "configured-token"); err != nil {
		t.Fatal(err)
	}
	acc, _ := h.Store.FindAccount("token@example.com")

	result := h.testAccount(context.Background(), acc, "deepseek-flash", "")
	if ok, _ := result["success"].(bool); ok {
		t.Fatalf("expected muted account to fail: %#v", result)
	}
	if msg, _ := result["message"].(string); !strings.Contains(msg, "禁言") {
		t.Fatalf("unexpected message: %q", msg)
	}
	if ds.loginCalls != 0 || ds.createSessionCalls != 0 {
		t.Fatalf("muted token should not login/create session: %#v", ds)
	}
}
