package admin

import (
	"fmt"
	"strings"

	"ds2api/internal/config"
)

func normalizeSettingsConfig(c *config.Config) {
	if c == nil {
		return
	}
	c.Admin.PasswordHash = strings.TrimSpace(c.Admin.PasswordHash)
	c.Embeddings.Provider = strings.TrimSpace(c.Embeddings.Provider)
}

func validateSettingsConfig(c config.Config) error {
	if c.Admin.JWTExpireHours != 0 && (c.Admin.JWTExpireHours < 1 || c.Admin.JWTExpireHours > 720) {
		return fmt.Errorf("admin.jwt_expire_hours must be between 1 and 720")
	}
	if err := validateRuntimeSettings(c.Runtime); err != nil {
		return err
	}
	if c.Responses.StoreTTLSeconds != 0 && (c.Responses.StoreTTLSeconds < 30 || c.Responses.StoreTTLSeconds > 86400) {
		return fmt.Errorf("responses.store_ttl_seconds must be between 30 and 86400")
	}
	if c.Embeddings.Provider != "" && strings.TrimSpace(c.Embeddings.Provider) == "" {
		return fmt.Errorf("embeddings.provider cannot be empty")
	}
	return nil
}

func validateRuntimeSettings(runtime config.RuntimeConfig) error {
	if runtime.AccountMaxInflight != 0 && (runtime.AccountMaxInflight < 1 || runtime.AccountMaxInflight > 256) {
		return fmt.Errorf("runtime.account_max_inflight must be between 1 and 256")
	}
	if runtime.AccountMaxQueue != 0 && (runtime.AccountMaxQueue < 1 || runtime.AccountMaxQueue > 200000) {
		return fmt.Errorf("runtime.account_max_queue must be between 1 and 200000")
	}
	if runtime.GlobalMaxInflight != 0 && (runtime.GlobalMaxInflight < 1 || runtime.GlobalMaxInflight > 200000) {
		return fmt.Errorf("runtime.global_max_inflight must be between 1 and 200000")
	}
	if runtime.TokenRefreshIntervalHours != 0 && (runtime.TokenRefreshIntervalHours < 1 || runtime.TokenRefreshIntervalHours > 720) {
		return fmt.Errorf("runtime.token_refresh_interval_hours must be between 1 and 720")
	}
	if runtime.AccountMaxInflight > 0 && runtime.GlobalMaxInflight > 0 && runtime.GlobalMaxInflight < runtime.AccountMaxInflight {
		return fmt.Errorf("runtime.global_max_inflight must be >= runtime.account_max_inflight")
	}
	return nil
}
