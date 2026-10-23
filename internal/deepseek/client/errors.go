package client

import (
	"errors"
	"fmt"
	"time"
)

type FailureKind string

const (
	FailureUnknown             FailureKind = ""
	FailureDirectUnauthorized  FailureKind = "direct_unauthorized"
	FailureManagedUnauthorized FailureKind = "managed_unauthorized"
	FailureAccountMuted        FailureKind = "account_muted"
	FailureRateLimited         FailureKind = "rate_limited"
	FailureRiskDevice          FailureKind = "risk_device"
)

type RequestFailure struct {
	Op         string
	Kind       FailureKind
	Message    string
	MuteUntil  time.Time
	RetryAfter time.Duration
}

func (e *RequestFailure) Error() string {
	if e == nil {
		return ""
	}
	switch {
	case e.Op != "" && e.Message != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	case e.Op != "":
		return e.Op + " failed"
	case e.Message != "":
		return e.Message
	default:
		return "request failed"
	}
}

func IsManagedUnauthorizedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureManagedUnauthorized
}

func IsDirectUnauthorizedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureDirectUnauthorized
}

func IsAccountMutedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureAccountMuted
}

func IsRateLimitedError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureRateLimited
}

func IsRiskDeviceError(err error) bool {
	var failure *RequestFailure
	return errors.As(err, &failure) && failure.Kind == FailureRiskDevice
}
