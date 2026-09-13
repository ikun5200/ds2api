package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"unicode"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func (c *Client) Login(ctx context.Context, acc config.Account) (string, error) {
	deviceID := loginDeviceID(acc)
	if deviceID == "" {
		return "", errors.New("DeepSeek login requires a website-issued device_id; run node scripts/deepseek-device.mjs and set the account device_id or DS2API_DEEPSEEK_DEVICE_ID")
	}
	clients := c.requestClientsForAccount(acc)
	platform := dsprotocol.LoginHeaders["x-client-platform"]
	if platform == "" {
		platform = "web"
	}
	payload := map[string]any{
		"email":     "",
		"mobile":    "",
		"area_code": "",
		"password":  acc.Password,
		"device_id": deviceID,
		"os":        platform,
	}
	if email := strings.TrimSpace(acc.Email); email != "" {
		payload["email"] = email
	} else if mobile := strings.TrimSpace(acc.Mobile); mobile != "" {
		loginMobile, areaCode := normalizeMobileForLogin(mobile)
		if loginMobile == "" {
			return "", errors.New("invalid login mobile")
		}
		payload["mobile"] = loginMobile
		payload["area_code"] = areaCode
	} else {
		return "", errors.New("missing email/mobile")
	}
	resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekLoginURL, dsprotocol.LoginHeaders, payload)
	if err != nil {
		return "", err
	}
	code, bizCode, msg, bizMsg := extractResponseStatus(resp)
	if status != http.StatusOK || code != 0 || bizCode != 0 {
		message := failureMessage(msg, bizMsg, "upstream rejected login")
		if strings.Contains(strings.ToUpper(message), "RISK_DEVICE_DETECTED") {
			message += "; renew the website device verification and update device_id (node scripts/deepseek-device.mjs)"
		}
		return "", fmt.Errorf("login failed (HTTP %d, code %d, biz_code %d): %s", status, code, bizCode, message)
	}
	data, _ := resp["data"].(map[string]any)
	bizData, _ := data["biz_data"].(map[string]any)
	user, _ := bizData["user"].(map[string]any)
	token, _ := user["token"].(string)
	if strings.TrimSpace(token) == "" {
		return "", errors.New("missing login token")
	}
	return token, nil
}

func loginDeviceID(acc config.Account) string {
	if override := strings.TrimSpace(os.Getenv("DS2API_DEEPSEEK_DEVICE_ID")); override != "" {
		return override
	}
	return strings.TrimSpace(acc.DeviceID)
}

func (c *Client) CreateSession(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = c.maxRetries
	}
	clients := c.requestClientsForAuth(ctx, a)
	attempts := 0
	refreshed := false
	for attempts < maxAttempts {
		headers := c.authHeaders(a.DeepSeekToken)
		resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekCreateSessionURL, headers, map[string]any{})
		if err != nil {
			config.Logger.Warn("[create_session] request error", "error", err, "account", a.AccountID)
			attempts++
			continue
		}
		code, bizCode, msg, bizMsg := extractResponseStatus(resp)
		if status == http.StatusOK && code == 0 && bizCode == 0 {
			sessionID := extractCreateSessionID(resp)
			if sessionID != "" {
				return sessionID, nil
			}
		}
		config.Logger.Warn("[create_session] failed", "status", status, "code", code, "biz_code", bizCode, "msg", msg, "biz_msg", bizMsg, "use_config_token", a.UseConfigToken, "account", a.AccountID)
		if a.UseConfigToken {
			if !refreshed && shouldAttemptRefresh(status, code, bizCode, msg, bizMsg) {
				if c.Auth.RefreshToken(ctx, a) {
					refreshed = true
					continue
				}
			}
			if c.Auth.SwitchAccount(ctx, a) {
				refreshed = false
				attempts++
				continue
			}
		}
		attempts++
	}
	return "", errors.New("create session failed")
}

func (c *Client) GetPow(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error) {
	return c.GetPowForTarget(ctx, a, dsprotocol.DeepSeekCompletionTargetPath, maxAttempts)
}

func (c *Client) GetPowForTarget(ctx context.Context, a *auth.RequestAuth, targetPath string, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = c.maxRetries
	}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		targetPath = dsprotocol.DeepSeekCompletionTargetPath
	}
	clients := c.requestClientsForAuth(ctx, a)
	attempts := 0
	refreshed := false
	lastFailureKind := FailureUnknown
	lastFailureMessage := ""
	for attempts < maxAttempts {
		headers := c.authHeaders(a.DeepSeekToken)
		resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekCreatePowURL, headers, map[string]any{"target_path": targetPath})
		if err != nil {
			config.Logger.Warn("[get_pow] request error", "error", err, "account", a.AccountID, "target_path", targetPath)
			lastFailureKind = FailureUnknown
			lastFailureMessage = err.Error()
			attempts++
			continue
		}
		code, bizCode, msg, bizMsg := extractResponseStatus(resp)
		if status == http.StatusOK && code == 0 && bizCode == 0 {
			data, _ := resp["data"].(map[string]any)
			bizData, _ := data["biz_data"].(map[string]any)
			challenge, _ := bizData["challenge"].(map[string]any)
			answer, err := ComputePow(ctx, challenge)
			if err != nil {
				attempts++
				continue
			}
			return BuildPowHeader(challenge, answer)
		}
		config.Logger.Warn("[get_pow] failed", "status", status, "code", code, "biz_code", bizCode, "msg", msg, "biz_msg", bizMsg, "use_config_token", a.UseConfigToken, "account", a.AccountID, "target_path", targetPath)
		lastFailureMessage = failureMessage(msg, bizMsg, "get pow failed")
		if isTokenInvalid(status, code, bizCode, msg, bizMsg) || isAuthIndicativeBizFailure(msg, bizMsg) {
			lastFailureKind = authFailureKind(a.UseConfigToken)
		} else {
			lastFailureKind = FailureUnknown
		}
		if a.UseConfigToken {
			if !refreshed && shouldAttemptRefresh(status, code, bizCode, msg, bizMsg) {
				if c.Auth.RefreshToken(ctx, a) {
					refreshed = true
					continue
				}
			}
			if c.Auth.SwitchAccount(ctx, a) {
				refreshed = false
				attempts++
				continue
			}
		}
		attempts++
	}
	if lastFailureKind != FailureUnknown {
		return "", &RequestFailure{Op: "get pow", Kind: lastFailureKind, Message: lastFailureMessage}
	}
	return "", errors.New("get pow failed")
}

func (c *Client) authHeaders(token string) map[string]string {
	headers := make(map[string]string, len(dsprotocol.BaseHeaders)+1)
	for k, v := range dsprotocol.BaseHeaders {
		headers[k] = v
	}
	headers["authorization"] = "Bearer " + token
	return headers
}

func isTokenInvalid(status int, code int, bizCode int, msg string, bizMsg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg) + " " + strings.TrimSpace(bizMsg))
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return true
	}
	if code == 40001 || code == 40002 || code == 40003 || bizCode == 40001 || bizCode == 40002 || bizCode == 40003 {
		return true
	}
	return strings.Contains(msg, "token") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "expired") ||
		strings.Contains(msg, "not login") ||
		strings.Contains(msg, "login required") ||
		strings.Contains(msg, "invalid jwt")
}

func shouldAttemptRefresh(status int, code int, bizCode int, msg string, bizMsg string) bool {
	if isTokenInvalid(status, code, bizCode, msg, bizMsg) {
		return true
	}
	// Some DeepSeek failures come back as HTTP 200/code=0 but with non-zero biz_code.
	// Only attempt refresh when these biz failures still look auth-related.
	return status == http.StatusOK &&
		code == 0 &&
		bizCode != 0 &&
		isAuthIndicativeBizFailure(msg, bizMsg)
}

func isAuthIndicativeBizFailure(msg string, bizMsg string) bool {
	combined := strings.ToLower(strings.TrimSpace(msg) + " " + strings.TrimSpace(bizMsg))
	authKeywords := []string{
		"auth",
		"authorization",
		"credential",
		"expired",
		"invalid jwt",
		"jwt",
		"login",
		"not login",
		"session expired",
		"token",
		"unauthorized",
		"登录",
		"未登录",
		"认证",
		"凭证",
		"会话过期",
		"令牌",
	}
	for _, keyword := range authKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}
	return false
}

func authFailureKind(useConfigToken bool) FailureKind {
	if useConfigToken {
		return FailureManagedUnauthorized
	}
	return FailureDirectUnauthorized
}

func failureMessage(msg string, bizMsg string, fallback string) string {
	if trimmed := strings.TrimSpace(bizMsg); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(msg); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

// DeepSeek has returned create-session ids in both biz_data.id and
// biz_data.chat_session.id across observed response variants; accept either.
func extractCreateSessionID(resp map[string]any) string {
	data, _ := resp["data"].(map[string]any)
	bizData, _ := data["biz_data"].(map[string]any)
	if sessionID, _ := bizData["id"].(string); strings.TrimSpace(sessionID) != "" {
		return strings.TrimSpace(sessionID)
	}
	if chatSession, ok := bizData["chat_session"].(map[string]any); ok {
		if sessionID, _ := chatSession["id"].(string); strings.TrimSpace(sessionID) != "" {
			return strings.TrimSpace(sessionID)
		}
	}
	return ""
}

func extractResponseStatus(resp map[string]any) (code int, bizCode int, msg string, bizMsg string) {
	code = intFrom(resp["code"])
	msg, _ = resp["msg"].(string)
	data, _ := resp["data"].(map[string]any)
	bizCode = intFrom(data["biz_code"])
	bizMsg, _ = data["biz_msg"].(string)
	if strings.TrimSpace(bizMsg) == "" {
		if bizData, ok := data["biz_data"].(map[string]any); ok {
			bizMsg, _ = bizData["msg"].(string)
		}
	}
	return code, bizCode, msg, bizMsg
}

func normalizeMobileForLogin(raw string) (mobile string, areaCode string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ""
	}
	hasPlus := strings.HasPrefix(s, "+")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return "", ""
	}
	if (hasPlus || strings.HasPrefix(digits, "86")) && strings.HasPrefix(digits, "86") && len(digits) == 13 {
		return digits[2:], "+86"
	}
	if len(digits) == 11 && strings.HasPrefix(digits, "1") && !hasPlus {
		return digits, "+86"
	}
	return digits, ""
}
