package client

import (
	"bytes"
	"context"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	trans "ds2api/internal/deepseek/transport"
)

func (c *Client) CallCompletion(ctx context.Context, a *auth.RequestAuth, payload map[string]any, powResp string, maxAttempts int) (*http.Response, error) {
	_ = maxAttempts
	clients := c.requestClientsForAuth(ctx, a)
	headers := c.authHeaders(a.DeepSeekToken)
	headers["x-ds-pow-response"] = powResp
	sessionID, _ := payload["chat_session_id"].(string)
	headers = withDeepSeekWebPageHeaders(headers, sessionID)
	captureSession := c.capture.Start("deepseek_completion", dsprotocol.DeepSeekCompletionURL, a.AccountID, payload)
	resp, err := c.streamPostOnce(ctx, clients.stream, dsprotocol.DeepSeekCompletionURL, headers, payload)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		closeErr := resp.Body.Close()
		if closeErr != nil {
			config.Logger.Warn("[completion] failed to close non-SSE response body", "error", closeErr)
		}
		if readErr != nil {
			return nil, readErr
		}
		parsed := map[string]any{}
		if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
			if info, muted := c.detectMute(a, parsed); muted {
				return nil, c.muteFailure("completion", info)
			}
			if isRateLimitedResponse(parsed) {
				return nil, &RequestFailure{
					Op:         "completion",
					Kind:       FailureRateLimited,
					Message:    failureMessage(asString(parsed["msg"]), "", "rate limit reached"),
					RetryAfter: time.Minute,
				}
			}
			if isRiskDeviceResponse(parsed) {
				return nil, &RequestFailure{
					Op:      "completion",
					Kind:    FailureRiskDevice,
					Message: failureMessage(asString(parsed["msg"]), "", "RISK_DEVICE_DETECTED"),
				}
			}
		}
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	if captureSession != nil {
		resp.Body = captureSession.WrapBody(resp.Body, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusOK {
		resp = c.wrapCompletionWithAutoContinue(ctx, a, payload, powResp, resp)
	}
	return resp, nil
}

func (c *Client) streamPost(ctx context.Context, doer trans.Doer, url string, headers map[string]string, payload any) (*http.Response, error) {
	return c.streamPostWithFallback(ctx, doer, url, headers, payload, true)
}

func (c *Client) streamPostOnce(ctx context.Context, doer trans.Doer, url string, headers map[string]string, payload any) (*http.Response, error) {
	return c.streamPostWithFallback(ctx, doer, url, headers, payload, false)
}

func (c *Client) streamPostWithFallback(ctx context.Context, doer trans.Doer, url string, headers map[string]string, payload any, allowFallback bool) (*http.Response, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	headers = c.jsonHeaders(headers)
	clients := c.requestClientsFromContext(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := doer.Do(req)
	if err != nil {
		if allowFallback {
			config.Logger.Warn("[deepseek] fingerprint stream request failed, fallback to std transport", "url", url, "error", err)
			req2, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
			if reqErr != nil {
				return nil, reqErr
			}
			for k, v := range headers {
				req2.Header.Set(k, v)
			}
			return clients.fallbackS.Do(req2)
		}
		return nil, err
	}
	return resp, nil
}
