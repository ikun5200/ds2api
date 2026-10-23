package history

import (
	"errors"
	"net/http"
	"time"

	dsclient "ds2api/internal/deepseek/client"
)

func MapError(err error) (int, string) {
	status, message, _, _ := MapErrorWithRetry(err)
	return status, message
}

// MapErrorWithRetry maps a DeepSeek client failure to an HTTP status, message,
// error code, and optional Retry-After duration.
func MapErrorWithRetry(err error) (int, string, string, time.Duration) {
	switch {
	case dsclient.IsManagedUnauthorizedError(err):
		return http.StatusUnauthorized, "Account token is invalid. Please re-login the account in admin.", "error", 0
	case dsclient.IsDirectUnauthorizedError(err):
		return http.StatusUnauthorized, "Invalid token. If this should be a DS2API key, add it to config.keys first.", "error", 0
	case dsclient.IsAccountMutedError(err):
		status, message, retryAfter := muteErrorDetail(err)
		return status, message, "account_muted", retryAfter
	case dsclient.IsRateLimitedError(err):
		status, message, retryAfter := rateLimitErrorDetail(err)
		return status, message, "rate_limited", retryAfter
	case dsclient.IsRiskDeviceError(err):
		return http.StatusForbidden, err.Error(), "risk_device", 0
	default:
		return http.StatusInternalServerError, err.Error(), "error", 0
	}
}

func muteErrorDetail(err error) (int, string, time.Duration) {
	var failure *dsclient.RequestFailure
	if errors.As(err, &failure) {
		message := failure.Message
		if message == "" {
			message = "Upstream account is muted"
		}
		if !failure.MuteUntil.IsZero() {
			message += " until " + failure.MuteUntil.Format(time.RFC3339)
		}
		retryAfter := failure.RetryAfter
		if retryAfter <= 0 && !failure.MuteUntil.IsZero() {
			retryAfter = time.Until(failure.MuteUntil)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}
		return http.StatusTooManyRequests, message, retryAfter
	}
	return http.StatusTooManyRequests, err.Error(), 0
}

func rateLimitErrorDetail(err error) (int, string, time.Duration) {
	var failure *dsclient.RequestFailure
	if errors.As(err, &failure) {
		message := failure.Message
		if message == "" {
			message = "Upstream rate limit reached"
		}
		retryAfter := failure.RetryAfter
		if retryAfter <= 0 {
			retryAfter = time.Minute
		}
		return http.StatusTooManyRequests, message, retryAfter
	}
	return http.StatusTooManyRequests, err.Error(), time.Minute
}
