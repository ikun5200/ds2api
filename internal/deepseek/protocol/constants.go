package protocol

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	DeepSeekHost                 = "chat.deepseek.com"
	DeepSeekOrigin               = "https://chat.deepseek.com"
	DeepSeekBaseReferer          = "https://chat.deepseek.com/"
	DeepSeekLoginURL             = "https://chat.deepseek.com/api/v0/users/login"
	DeepSeekCreateSessionURL     = "https://chat.deepseek.com/api/v0/chat_session/create"
	DeepSeekCreatePowURL         = "https://chat.deepseek.com/api/v0/chat/create_pow_challenge"
	DeepSeekCompletionURL        = "https://chat.deepseek.com/api/v0/chat/completion"
	DeepSeekContinueURL          = "https://chat.deepseek.com/api/v0/chat/continue"
	DeepSeekUploadFileURL        = "https://chat.deepseek.com/api/v0/file/upload_file"
	DeepSeekFetchFilesURL        = "https://chat.deepseek.com/api/v0/file/fetch_files"
	DeepSeekFetchSessionURL      = "https://chat.deepseek.com/api/v0/chat_session/fetch_page"
	DeepSeekDeleteSessionURL     = "https://chat.deepseek.com/api/v0/chat_session/delete"
	DeepSeekDeleteAllSessionsURL = "https://chat.deepseek.com/api/v0/chat_session/delete_all"
	DeepSeekCompletionTargetPath = "/api/v0/chat/completion"
	DeepSeekUploadTargetPath     = "/api/v0/file/upload_file"
)

var defaultStaticBaseHeaders = map[string]string{
	"Host":            "chat.deepseek.com",
	"Accept":          "*/*",
	"Accept-Language": "zh-CN,zh;q=0.9",
	"Content-Type":    "application/json",
}

var defaultSkipContainsPatterns = []string{
	"quasi_status",
	"elapsed_secs",
	"token_usage",
	"pending_fragment",
	"conversation_mode",
	"fragments/-1/status",
	"fragments/-2/status",
	"fragments/-3/status",
}

var defaultSkipExactPaths = []string{
	"response/search_status",
}

var ClientVersion string
var BaseHeaders = map[string]string{}
var SkipContainsPatterns = cloneStringSlice(defaultSkipContainsPatterns)
var SkipExactPathSet = toStringSet(defaultSkipExactPaths)

type clientConstants struct {
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	Version         string `json:"version"`
	AndroidAPILevel string `json:"android_api_level"`
	Locale          string `json:"locale"`
}

type sharedConstants struct {
	Client              clientConstants   `json:"client"`
	BaseHeaders         map[string]string `json:"base_headers"`
	SkipContainsPattern []string          `json:"skip_contains_patterns"`
	SkipExactPaths      []string          `json:"skip_exact_paths"`
}

//go:embed constants_shared.json
var sharedConstantsJSON []byte

func init() {
	cfg := sharedConstants{}
	if err := json.Unmarshal(sharedConstantsJSON, &cfg); err != nil {
		panic(fmt.Errorf("load DeepSeek shared constants: %w", err))
	}
	applySharedConstants(cfg)
}

func applySharedConstants(cfg sharedConstants) {
	client := normalizeClientConstants(cfg.Client)
	applyClientEnvOverrides(&client)
	ClientVersion = client.Version
	BaseHeaders = applyBaseHeaderEnvOverrides(buildBaseHeaders(client, cfg.BaseHeaders))
	SkipContainsPatterns = cloneStringSlice(defaultSkipContainsPatterns)
	if len(cfg.SkipContainsPattern) > 0 {
		SkipContainsPatterns = cloneStringSlice(cfg.SkipContainsPattern)
	}
	SkipExactPathSet = toStringSet(defaultSkipExactPaths)
	if len(cfg.SkipExactPaths) > 0 {
		SkipExactPathSet = toStringSet(cfg.SkipExactPaths)
	}
}

func normalizeClientConstants(in clientConstants) clientConstants {
	if in.Name == "" {
		in.Name = "DeepSeek"
	}
	if in.Platform == "" {
		in.Platform = "web"
	}
	if in.AndroidAPILevel == "" && strings.EqualFold(in.Platform, "android") {
		in.AndroidAPILevel = "35"
	}
	if in.Locale == "" {
		in.Locale = "zh_CN"
	}
	return in
}

func applyClientEnvOverrides(client *clientConstants) {
	if client == nil {
		return
	}
	if v := envString("DS2API_DEEPSEEK_CLIENT_NAME"); v != "" {
		client.Name = v
	}
	if v := envString("DS2API_DEEPSEEK_CLIENT_PLATFORM"); v != "" {
		client.Platform = strings.ToLower(v)
	}
	if v := envString("DS2API_DEEPSEEK_CLIENT_VERSION"); v != "" {
		client.Version = v
	}
	if v := envString("DS2API_DEEPSEEK_ANDROID_API_LEVEL"); v != "" {
		client.AndroidAPILevel = v
	}
	if v := envString("DS2API_DEEPSEEK_CLIENT_LOCALE"); v != "" {
		client.Locale = v
	}
}

func applyBaseHeaderEnvOverrides(headers map[string]string) map[string]string {
	out := cloneStringMap(headers)
	setHeaderFromEnv(out, "User-Agent", "DS2API_DEEPSEEK_USER_AGENT")
	setHeaderFromEnv(out, "Accept-Language", "DS2API_DEEPSEEK_ACCEPT_LANGUAGE")
	setHeaderFromEnv(out, "x-client-locale", "DS2API_DEEPSEEK_CLIENT_LOCALE")
	return out
}

func setHeaderFromEnv(headers map[string]string, key string, envName string) {
	if headers == nil {
		return
	}
	if v := envString(envName); v != "" {
		headers[key] = v
	}
}

func buildBaseHeaders(client clientConstants, overrides map[string]string) map[string]string {
	out := cloneStringMap(defaultStaticBaseHeaders)
	for k, v := range overrides {
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if client.Platform == "android" && client.Name != "" && client.Version != "" {
		userAgent := client.Name + "/" + client.Version
		if client.AndroidAPILevel != "" {
			userAgent += " Android/" + client.AndroidAPILevel
		}
		out["User-Agent"] = userAgent
	}
	if client.Platform != "" {
		out["x-client-platform"] = client.Platform
	}
	if client.Version != "" {
		out["x-client-version"] = client.Version
		if client.Platform == "web" || out["x-app-version"] != "" {
			out["x-app-version"] = client.Version
		}
	}
	if client.Locale != "" {
		out["x-client-locale"] = client.Locale
	}
	return out
}

func SessionReferer(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return DeepSeekBaseReferer
	}
	return DeepSeekOrigin + "/a/chat/s/" + sessionID
}

func envString(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringSlice(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func toStringSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, v := range in {
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

const (
	KeepAliveTimeout  = 5
	StreamIdleTimeout = 300
	MaxKeepaliveCount = 40
)
