package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	dsclient "ds2api/internal/deepseek/client"
)

type tokenProfileFetcher interface {
	GetCurrentUserForToken(ctx context.Context, token string) (*dsclient.AccountProfile, error)
}

func (h *Handler) fetchTokenProfile(ctx context.Context, token string) (*dsclient.AccountProfile, error) {
	fetcher, ok := h.DS.(tokenProfileFetcher)
	if !ok {
		return nil, errors.New("当前 DeepSeek 客户端不支持 token 校验")
	}
	return fetcher.GetCurrentUserForToken(ctx, token)
}

func (h *Handler) tokenInfo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "token 不能为空"})
		return
	}
	profile, err := h.fetchTokenProfile(r.Context(), token)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Token 校验失败: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":              true,
		"token_valid":          true,
		"user_id":              profile.UserID,
		"email":                profile.Email,
		"mobile":               profile.Mobile,
		"area_code":            profile.AreaCode,
		"name":                 profile.Name,
		"suggested_identifier": profile.Identifier(),
		"suggested_name":       profile.DisplayName(),
		"muted":                profile.Muted,
		"mute_until":           unixOrZero(profile.MuteUntil),
	})
}
