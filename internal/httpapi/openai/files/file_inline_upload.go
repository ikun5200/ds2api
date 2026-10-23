package files

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"ds2api/internal/auth"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/inputfiles"
)

func (h *Handler) PreprocessInlineFileInputs(ctx context.Context, a *auth.RequestAuth, req map[string]any) error {
	if h == nil {
		return nil
	}
	return (&inputfiles.Service{Store: h.Store, DS: h.DS}).PreprocessInlineFileInputs(ctx, a, req)
}

func WriteInlineFileError(w http.ResponseWriter, err error) {
	status, message, retryAfter := inputfiles.MapErrorWithRetry(err)
	setFileRetryAfter(w, retryAfter)
	shared.WriteOpenAIError(w, status, message)
}

func setFileRetryAfter(w http.ResponseWriter, d time.Duration) {
	if d <= 0 {
		return
	}
	seconds := int(d.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
