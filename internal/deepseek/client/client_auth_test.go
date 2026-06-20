package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestLoginDeviceIDUsesStableAccountDerivedValue(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_SEED", "test-seed")
	acc := config.Account{Email: "USER@example.com"}

	first := loginDeviceID(acc)
	second := loginDeviceID(acc)
	if first == "" {
		t.Fatal("expected non-empty device id")
	}
	if first != second {
		t.Fatalf("expected stable device id, got %q then %q", first, second)
	}
	if first == "deepseek_to_api" || strings.Contains(first, "ds2api") {
		t.Fatalf("device id still exposes project fingerprint: %q", first)
	}
	if len(first) != 32 {
		t.Fatalf("expected 16-byte hex device id, got %q", first)
	}
}

func TestLoginDeviceIDSupportsExplicitOverride(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "device-override")

	got := loginDeviceID(config.Account{Email: "user@example.com"})
	if got != "device-override" {
		t.Fatalf("expected override device id, got %q", got)
	}
}

func TestCreateSessionUsesWebEmptyRequestBody(t *testing.T) {
	var seenBody string
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			bodyBytes, _ := io.ReadAll(req.Body)
			seenBody = string(bodyBytes)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":0,
					"data":{"biz_code":0,"biz_data":{"chat_session":{"id":"session-web"}}}
				}`)),
				Request: req,
			}, nil
		}),
		maxRetries: 1,
	}

	got, err := client.CreateSession(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, 1)
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if got != "session-web" {
		t.Fatalf("expected session-web, got %q", got)
	}
	if seenBody != "{}" {
		t.Fatalf("expected empty web session body {}, got %q", seenBody)
	}
}

func TestLoginUsesAndroidLoginHeaders(t *testing.T) {
	var seenPlatform string
	var seenUserAgent string
	var seenAccept string
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			seenPlatform = req.Header.Get("x-client-platform")
			seenUserAgent = req.Header.Get("User-Agent")
			seenAccept = req.Header.Get("Accept")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":0,
					"data":{"biz_code":0,"biz_data":{"user":{"token":"login-token"}}}
				}`)),
				Request: req,
			}, nil
		}),
	}

	got, err := client.Login(context.Background(), config.Account{Email: "user@example.com", Password: "pass"})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if got != "login-token" {
		t.Fatalf("expected login-token, got %q", got)
	}
	if seenPlatform != "android" {
		t.Fatalf("login x-client-platform=%q want android", seenPlatform)
	}
	if seenUserAgent != dsprotocol.LoginHeaders["User-Agent"] {
		t.Fatalf("login User-Agent=%q want %q", seenUserAgent, dsprotocol.LoginHeaders["User-Agent"])
	}
	if seenAccept != "application/json" {
		t.Fatalf("login Accept=%q want application/json", seenAccept)
	}
}
