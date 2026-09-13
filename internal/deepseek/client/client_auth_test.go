package client

import (
	"context"
	"encoding/json"
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

func TestLoginDeviceIDUsesAccountVerification(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "")
	if got := loginDeviceID(config.Account{Email: "user@example.com", DeviceID: " verified-device-id "}); got != "verified-device-id" {
		t.Fatalf("expected stored website device ID, got %q", got)
	}
	if got := loginDeviceID(config.Account{Email: "user@example.com"}); got != "" {
		t.Fatalf("must not manufacture an unregistered device ID: %q", got)
	}
}

func TestLoginDeviceIDSupportsExplicitOverride(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "device-override")

	got := loginDeviceID(config.Account{Email: "user@example.com", DeviceID: "account-device"})
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

func TestLoginUsesWebLoginRequest(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "website-issued-device-id")
	var seenPlatform string
	var seenUserAgent string
	var seenAccept string
	var seenBody map[string]any
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			seenPlatform = req.Header.Get("x-client-platform")
			seenUserAgent = req.Header.Get("User-Agent")
			seenAccept = req.Header.Get("Accept")
			if err := json.NewDecoder(req.Body).Decode(&seenBody); err != nil {
				t.Fatalf("decode login request: %v", err)
			}
			if req.Header.Get("Origin") != dsprotocol.DeepSeekOrigin || req.Header.Get("Referer") != dsprotocol.DeepSeekOrigin+"/sign_in" {
				t.Fatal("login must carry the website origin and sign-in referer")
			}
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

	got, err := client.Login(context.Background(), config.Account{Email: " user@example.com ", Password: " pass "})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if got != "login-token" {
		t.Fatalf("expected login-token, got %q", got)
	}
	if seenPlatform != "web" {
		t.Fatalf("login x-client-platform=%q want web", seenPlatform)
	}
	if seenUserAgent != dsprotocol.LoginHeaders["User-Agent"] {
		t.Fatalf("login User-Agent=%q want %q", seenUserAgent, dsprotocol.LoginHeaders["User-Agent"])
	}
	if seenAccept != "*/*" {
		t.Fatalf("login Accept=%q want */*", seenAccept)
	}
	for key, want := range map[string]string{
		"email": "user@example.com", "password": " pass ", "mobile": "", "area_code": "", "os": "web", "device_id": "website-issued-device-id",
	} {
		if seenBody[key] != want {
			t.Errorf("login body[%s]=%v want %q", key, seenBody[key], want)
		}
	}
}

func TestLoginRejectsHTTPFailureEvenWithSuccessBody(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "website-issued-device-id")
	client := &Client{regular: doerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"biz_code":0,"biz_data":{"user":{"token":"invalid-token"}}}}`)), Request: req}, nil
	})}
	token, err := client.Login(context.Background(), config.Account{Email: "user@example.com", Password: "pass"})
	if token != "" || err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("HTTP failure was accepted: token=%q err=%v", token, err)
	}
}

func TestLoginMissingDeviceDoesNotSendCredentials(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "")
	client := &Client{regular: doerFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("login without website device verification must not send credentials")
		return nil, nil
	})}
	_, err := client.Login(context.Background(), config.Account{Email: "user@example.com", Password: "secret"})
	if err == nil || !strings.Contains(err.Error(), "device_id") || !strings.Contains(err.Error(), "deepseek-device.mjs") {
		t.Fatalf("missing actionable device verification error: %v", err)
	}
}

func TestLoginDeviceRejectionIncludesRenewalGuidance(t *testing.T) {
	t.Setenv("DS2API_DEEPSEEK_DEVICE_ID", "")
	client := &Client{regular: doerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"biz_code":11,"biz_msg":"RISK_DEVICE_DETECTED"}}`)), Request: req}, nil
	})}
	_, err := client.Login(context.Background(), config.Account{Email: "user@example.com", Password: "secret", DeviceID: "stale-device"})
	if err == nil || !strings.Contains(err.Error(), "RISK_DEVICE_DETECTED") || !strings.Contains(err.Error(), "renew") {
		t.Fatalf("missing device renewal error: %v", err)
	}
}
