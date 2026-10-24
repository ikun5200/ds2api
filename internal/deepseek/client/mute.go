package client

import (
	"context"
	"errors"
	"strings"
	"time"

	"ds2api/internal/auth"
	"ds2api/internal/config"
)

// MuteInfo is the upstream risk-control mute signal carried in biz_data.
type MuteInfo struct {
	Muted  bool
	Until  time.Time
	Reason string
}

func extractMuteInfo(parsed map[string]any) MuteInfo {
	if parsed == nil {
		return MuteInfo{}
	}
	data, _ := parsed["data"].(map[string]any)
	if data == nil {
		return MuteInfo{}
	}
	bizData, _ := data["biz_data"].(map[string]any)
	if bizData == nil {
		bizData = map[string]any{}
	}
	reason := strings.TrimSpace(asString(data["biz_msg"]))
	if reason == "" {
		reason = strings.TrimSpace(asString(bizData["msg"]))
	}
	chat, _ := bizData["chat"].(map[string]any)
	muted := intFrom(bizData["is_muted"]) == 1 || intFrom(data["biz_code"]) == 14 ||
		(chat != nil && intFrom(chat["is_muted"]) == 1)
	if !muted {
		lower := strings.ToLower(reason)
		muted = strings.Contains(lower, "user is muted")
	}
	if !muted {
		return MuteInfo{}
	}
	var until time.Time
	if seconds := intFrom(bizData["mute_until"]); seconds > 0 {
		until = time.Unix(int64(seconds), 0)
	} else if chat != nil {
		if seconds := intFrom(chat["mute_until"]); seconds > 0 {
			until = time.Unix(int64(seconds), 0)
		}
	}
	if reason == "" {
		reason = "user is muted"
	}
	return MuteInfo{Muted: true, Until: until, Reason: reason}
}

func isRateLimitedResponse(parsed map[string]any) bool {
	if parsed == nil {
		return false
	}
	data, _ := parsed["data"].(map[string]any)
	bizCode := intFrom(data["biz_code"])
	message := strings.ToLower(strings.TrimSpace(asString(data["biz_msg"])))
	if message == "" {
		message = strings.ToLower(strings.TrimSpace(asString(parsed["msg"])))
	}
	return bizCode == 7 || strings.Contains(message, "rate limit") || strings.Contains(message, "too many requests")
}

func isRiskDeviceResponse(parsed map[string]any) bool {
	if parsed == nil {
		return false
	}
	data, _ := parsed["data"].(map[string]any)
	message := strings.ToUpper(strings.TrimSpace(asString(data["biz_msg"])))
	if message == "" {
		message = strings.ToUpper(strings.TrimSpace(asString(parsed["msg"])))
	}
	return strings.Contains(message, "RISK_DEVICE_DETECTED")
}

func (c *Client) markMuted(a *auth.RequestAuth, info MuteInfo) {
	if c == nil || c.Auth == nil || !info.Muted {
		return
	}
	c.Auth.MarkAuthMuted(a, info.Until, info.Reason)
}

func (c *Client) markMutedAccount(accountID string, info MuteInfo) {
	if c == nil || c.Auth == nil || !info.Muted {
		return
	}
	c.Auth.MarkAccountMuted(accountID, info.Until, info.Reason)
}

func (c *Client) detectMute(a *auth.RequestAuth, parsed map[string]any) (MuteInfo, bool) {
	info := extractMuteInfo(parsed)
	if info.Muted {
		c.markMuted(a, info)
	}
	return info, info.Muted
}

func (c *Client) detectMuteForAccount(accountID string, parsed map[string]any) (MuteInfo, bool) {
	info := extractMuteInfo(parsed)
	if info.Muted {
		c.markMutedAccount(accountID, info)
	}
	return info, info.Muted
}

func (c *Client) muteFailure(op string, info MuteInfo) *RequestFailure {
	return &RequestFailure{
		Op:        op,
		Kind:      FailureAccountMuted,
		Message:   info.Reason,
		MuteUntil: info.Until,
	}
}

// ProbeAccountMute performs a tiny upload as a mute probe. Upload is the
// endpoint that reports account-level mute with mute_until, while session
// creation can still succeed for muted accounts.
func (c *Client) ProbeAccountMute(ctx context.Context, acc config.Account) (bool, time.Time, error) {
	token, err := c.Login(ctx, acc)
	if err != nil {
		return false, time.Time{}, err
	}
	if token != "" {
		_ = c.Store.UpdateAccountToken(acc.Identifier(), token)
	}
	a := &auth.RequestAuth{
		UseConfigToken: false,
		DeepSeekToken:  token,
		AccountID:      acc.Identifier(),
		Account:        acc,
	}
	disabled := false
	_, err = c.UploadFile(ctx, a, UploadFileRequest{
		Filename:        "mute-probe.txt",
		ContentType:     "text/plain; charset=utf-8",
		Purpose:         "assistants",
		Data:            []byte("."),
		ThinkingEnabled: &disabled,
	}, 1)
	if err == nil {
		return false, time.Time{}, nil
	}
	var failure *RequestFailure
	if errors.As(err, &failure) && failure.Kind == FailureAccountMuted {
		return true, failure.MuteUntil, nil
	}
	return false, time.Time{}, err
}

// RunMuteProber periodically re-checks muted accounts at their scheduled probe
// time. It runs until ctx is cancelled.
func (c *Client) RunMuteProber(ctx context.Context) {
	if c == nil || c.Auth == nil || c.Auth.Pool == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.probeDueMutedAccounts(ctx)
		}
	}
}

func (c *Client) probeDueMutedAccounts(ctx context.Context) {
	due := c.Auth.Pool.MutedAccountsDue(time.Now())
	for _, accountID := range due {
		acc, ok := c.Store.FindAccount(accountID)
		if !ok {
			c.Auth.Pool.ClearMute(accountID)
			continue
		}
		probeCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		muted, until, err := c.ProbeAccountMute(probeCtx, acc)
		cancel()
		switch {
		case err != nil:
			config.Logger.Warn("[mute_probe] failed", "account", accountID, "error", err)
			c.Auth.Pool.MarkProbeResult(accountID, true, time.Time{}, "", err)
		case muted:
			config.Logger.Info("[mute_probe] still muted", "account", accountID, "mute_until", until.Format(time.RFC3339))
			c.Auth.Pool.MarkProbeResult(accountID, true, until, "user is muted", nil)
		default:
			config.Logger.Info("[mute_probe] unmuted", "account", accountID)
			c.Auth.Pool.MarkProbeResult(accountID, false, time.Time{}, "", nil)
		}
	}
}
