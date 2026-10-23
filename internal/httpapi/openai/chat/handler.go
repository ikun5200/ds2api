package chat

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"ds2api/internal/auth"
	"ds2api/internal/chathistory"
	"ds2api/internal/httpapi/openai/files"
	"ds2api/internal/httpapi/openai/history"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/promptcompat"
	"ds2api/internal/textclean"
	"ds2api/internal/toolcall"
	"ds2api/internal/toolstream"
)

const openAIGeneralMaxSize = shared.GeneralMaxSize

var writeJSON = shared.WriteJSON

type Handler struct {
	Store       shared.ConfigReader
	Auth        shared.AuthResolver
	DS          shared.DeepSeekCaller
	ChatHistory *chathistory.Store
}

func stripReferenceMarkersEnabled() bool {
	return textclean.StripReferenceMarkersEnabled()
}

func (h *Handler) applyCurrentInputFile(ctx context.Context, a *auth.RequestAuth, stdReq promptcompat.StandardRequest) (promptcompat.StandardRequest, error) {
	if h == nil {
		return stdReq, nil
	}
	stdReq = shared.ApplyThinkingInjection(h.Store, stdReq)
	svc := history.Service{Store: h.Store, DS: h.DS}
	out, err := svc.ApplyCurrentInputFile(ctx, a, stdReq)
	if err != nil || out.CurrentInputFileApplied {
		return out, err
	}
	return out, nil
}

func (h *Handler) preprocessInlineFileInputs(ctx context.Context, a *auth.RequestAuth, req map[string]any) error {
	if h == nil {
		return nil
	}
	return (&files.Handler{Store: h.Store, Auth: h.Auth, DS: h.DS, ChatHistory: h.ChatHistory}).PreprocessInlineFileInputs(ctx, a, req)
}

func (h *Handler) toolcallFeatureMatchEnabled() bool {
	if h == nil {
		return shared.ToolcallFeatureMatchEnabled(nil)
	}
	return shared.ToolcallFeatureMatchEnabled(h.Store)
}

func (h *Handler) toolcallEarlyEmitHighConfidence() bool {
	if h == nil {
		return shared.ToolcallEarlyEmitHighConfidence(nil)
	}
	return shared.ToolcallEarlyEmitHighConfidence(h.Store)
}

func writeOpenAIError(w http.ResponseWriter, status int, message string) {
	shared.WriteOpenAIError(w, status, message)
}

func writeOpenAIErrorWithCode(w http.ResponseWriter, status int, message, code string) {
	shared.WriteOpenAIErrorWithCode(w, status, message, code)
}

func setRetryAfter(w http.ResponseWriter, d time.Duration) {
	if d <= 0 {
		return
	}
	seconds := int(d.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func writeAuthDetermineError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var allMuted *auth.AllAccountsMutedError
	var accountMuted *auth.AccountMutedError
	switch {
	case errors.As(err, &allMuted):
		setRetryAfter(w, allMuted.RetryAfter)
		writeOpenAIErrorWithCode(w, http.StatusTooManyRequests, err.Error(), "account_muted")
	case errors.As(err, &accountMuted):
		setRetryAfter(w, accountMuted.RetryAfter)
		writeOpenAIErrorWithCode(w, http.StatusTooManyRequests, err.Error(), "account_muted")
	case err == auth.ErrNoAccount:
		writeOpenAIErrorWithCode(w, http.StatusTooManyRequests, err.Error(), "rate_limit_exceeded")
	default:
		writeOpenAIError(w, http.StatusUnauthorized, err.Error())
	}
	return true
}

func openAIErrorType(status int) string {
	return shared.OpenAIErrorType(status)
}

func writeOpenAIInlineFileError(w http.ResponseWriter, err error) {
	files.WriteInlineFileError(w, err)
}

func mapCurrentInputFileError(err error) (int, string, string, time.Duration) {
	return history.MapErrorWithRetry(err)
}

func requestTraceID(r *http.Request) string {
	return shared.RequestTraceID(r)
}

func asString(v any) string {
	return shared.AsString(v)
}

func cleanVisibleOutput(text string, stripReferenceMarkers bool) string {
	return shared.CleanVisibleOutput(text, stripReferenceMarkers)
}

func emptyOutputRetryEnabled() bool {
	return shared.EmptyOutputRetryEnabled()
}

func emptyOutputRetryMaxAttempts() int {
	return shared.EmptyOutputRetryMaxAttempts()
}

func formatIncrementalStreamToolCallDeltas(deltas []toolstream.ToolCallDelta, ids map[int]string) []map[string]any {
	return shared.FormatIncrementalStreamToolCallDeltas(deltas, ids)
}

func filterIncrementalToolCallDeltasByAllowed(deltas []toolstream.ToolCallDelta, seenNames map[int]string) []toolstream.ToolCallDelta {
	return shared.FilterIncrementalToolCallDeltasByAllowed(deltas, seenNames)
}

func formatFinalStreamToolCallsWithStableIDs(calls []toolcall.ParsedToolCall, ids map[int]string, toolsRaw any) []map[string]any {
	return shared.FormatFinalStreamToolCallsWithStableIDs(calls, ids, toolsRaw)
}
