package files

import (
	"context"
	"net/http"

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
	status, message := inputfiles.MapError(err)
	shared.WriteOpenAIError(w, status, message)
}
