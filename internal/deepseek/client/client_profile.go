package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	dsprotocol "ds2api/internal/deepseek/protocol"
)

// AccountProfile is the account metadata returned by DeepSeek for a token.
// The email/mobile values may be masked by upstream; they are still stable
// identifiers for the same account.
type AccountProfile struct {
	UserID    string
	Token     string
	Email     string
	Mobile    string
	AreaCode  string
	Name      string
	Status    int
	Muted     bool
	MuteUntil time.Time
}

// Identifier returns a stable account identifier derived from the profile.
func (p AccountProfile) Identifier() string {
	if email := strings.TrimSpace(p.Email); email != "" {
		return email
	}
	if mobile := strings.TrimSpace(p.Mobile); mobile != "" {
		return mobile
	}
	if id := strings.TrimSpace(p.UserID); id != "" {
		return "token-" + id + "@local"
	}
	return ""
}

// DisplayName returns a human-readable label for the account.
func (p AccountProfile) DisplayName() string {
	if name := strings.TrimSpace(p.Name); name != "" {
		return name
	}
	if email := strings.TrimSpace(p.Email); email != "" {
		return email
	}
	if mobile := strings.TrimSpace(p.Mobile); mobile != "" {
		return mobile
	}
	if id := strings.TrimSpace(p.UserID); id != "" {
		if len(id) > 8 {
			id = id[:8]
		}
		return "DeepSeek " + id
	}
	return "DeepSeek Token"
}

// GetCurrentUserForToken validates a website token and returns account metadata.
// It uses the same token-authenticated endpoint as the web client.
func (c *Client) GetCurrentUserForToken(ctx context.Context, token string) (*AccountProfile, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("token is required")
	}
	clients := c.requestClientsFromContext(ctx)
	resp, status, err := c.getJSONWithStatus(ctx, clients.regular, dsprotocol.DeepSeekCurrentUserURL, c.authHeaders(token))
	if err != nil {
		return nil, err
	}
	code, bizCode, msg, bizMsg := extractResponseStatus(resp)
	if status != http.StatusOK || code != 0 || bizCode != 0 {
		return nil, fmt.Errorf("token validation failed (HTTP %d, code %d, biz_code %d): %s",
			status, code, bizCode, failureMessage(msg, bizMsg, "invalid token"))
	}
	data, _ := resp["data"].(map[string]any)
	bizData, _ := data["biz_data"].(map[string]any)
	if bizData == nil {
		return nil, errors.New("token validation failed: empty profile")
	}
	profile := &AccountProfile{
		UserID:   asString(bizData["id"]),
		Token:    asString(bizData["token"]),
		Email:    asString(bizData["email"]),
		Mobile:   asString(bizData["mobile_number"]),
		AreaCode: asString(bizData["area_code"]),
		Status:   intFrom(bizData["status"]),
		Name:     firstNonEmpty(asString(bizData["name"]), asString(bizData["nickname"]), asString(bizData["user_name"])),
	}
	if profile.Name == "" {
		if idProfile, ok := bizData["id_profile"].(map[string]any); ok {
			profile.Name = firstNonEmpty(asString(idProfile["name"]), asString(idProfile["nickname"]))
		}
	}
	if info := extractMuteInfo(resp); info.Muted {
		profile.Muted = true
		profile.MuteUntil = info.Until
	}
	if profile.Token == "" {
		profile.Token = token
	}
	return profile, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
