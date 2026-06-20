package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestCallCompletionDoesNotFallbackForNonIdempotentCompletion(t *testing.T) {
	var fallbackCalled bool
	client := &Client{
		stream: doerFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("ambiguous completion write failure")
		}),
		fallbackS: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			fallbackCalled = true
			return &http.Response{StatusCode: http.StatusOK}, nil
		})},
	}
	_, err := client.CallCompletion(
		context.Background(),
		&auth.RequestAuth{DeepSeekToken: "token"},
		map[string]any{"prompt": "hello"},
		"pow",
		3,
	)
	if err == nil {
		t.Fatal("expected completion error")
	}
	if fallbackCalled {
		t.Fatal("completion fallback should not be called for a non-idempotent request")
	}
}

func TestCallCompletionUsesWebPageHeaders(t *testing.T) {
	var seenOrigin string
	var seenReferer string
	var seenAccept string
	var seenPlatform string
	client := &Client{
		stream: doerFunc(func(req *http.Request) (*http.Response, error) {
			seenOrigin = req.Header.Get("Origin")
			seenReferer = req.Header.Get("Referer")
			seenAccept = req.Header.Get("Accept")
			seenPlatform = req.Header.Get("x-client-platform")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("data: [DONE]\n")),
				Request:    req,
			}, nil
		}),
	}
	resp, err := client.CallCompletion(
		context.Background(),
		&auth.RequestAuth{DeepSeekToken: "token"},
		map[string]any{"chat_session_id": "session-123", "prompt": "hello"},
		"pow",
		1,
	)
	if err != nil {
		t.Fatalf("CallCompletion returned error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if seenOrigin != dsprotocol.DeepSeekOrigin {
		t.Fatalf("completion Origin=%q want=%q", seenOrigin, dsprotocol.DeepSeekOrigin)
	}
	if seenReferer != dsprotocol.SessionReferer("session-123") {
		t.Fatalf("completion Referer=%q want=%q", seenReferer, dsprotocol.SessionReferer("session-123"))
	}
	if seenAccept != "*/*" {
		t.Fatalf("completion Accept=%q want */*", seenAccept)
	}
	if seenPlatform != "web" {
		t.Fatalf("completion x-client-platform=%q want web", seenPlatform)
	}
}
