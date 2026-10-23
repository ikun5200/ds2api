package completionruntime

import (
	"errors"
	"net/http"
	"time"

	"ds2api/internal/assistantturn"
	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

func deepSeekOutputError(err error) *assistantturn.OutputError {
	var failure *dsclient.RequestFailure
	if !errors.As(err, &failure) {
		return nil
	}
	switch failure.Kind {
	case dsclient.FailureAccountMuted:
		retryAfter := failure.RetryAfter
		if retryAfter <= 0 && !failure.MuteUntil.IsZero() {
			retryAfter = time.Until(failure.MuteUntil)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}
		message := "Upstream account is muted"
		if !failure.MuteUntil.IsZero() {
			message += " until " + failure.MuteUntil.Format(time.RFC3339)
		}
		return &assistantturn.OutputError{
			Status:     http.StatusTooManyRequests,
			Message:    message,
			Code:       "account_muted",
			RetryAfter: retryAfter,
		}
	case dsclient.FailureRateLimited:
		retryAfter := failure.RetryAfter
		if retryAfter <= 0 {
			retryAfter = time.Minute
		}
		return &assistantturn.OutputError{
			Status:     http.StatusTooManyRequests,
			Message:    failure.Message,
			Code:       "rate_limited",
			RetryAfter: retryAfter,
		}
	case dsclient.FailureRiskDevice:
		return &assistantturn.OutputError{
			Status:  http.StatusForbidden,
			Message: failure.Message,
			Code:    "risk_device",
		}
	}
	return nil
}

func startOutputError(err error, a *auth.RequestAuth) *assistantturn.OutputError {
	if out := deepSeekOutputError(err); out != nil {
		return out
	}
	return authOutputError(a)
}
