package files

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/auth"
	"ds2api/internal/chathistory"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/inputfiles"
	"ds2api/internal/util"
)

const openAIUploadMaxMemory = 32 << 20

type Handler struct {
	Store       shared.ConfigReader
	Auth        shared.AuthResolver
	DS          shared.DeepSeekCaller
	ChatHistory *chathistory.Store
}

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	a, err := h.Auth.Determine(r)
	if err != nil {
		status := http.StatusUnauthorized
		detail := err.Error()
		if err == auth.ErrNoAccount {
			status = http.StatusTooManyRequests
		}
		shared.WriteOpenAIError(w, status, detail)
		return
	}
	defer h.Auth.Release(a)
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type"))), "multipart/form-data") {
		shared.WriteOpenAIError(w, http.StatusBadRequest, "content-type must be multipart/form-data")
		return
	}
	// Enforce a hard cap on the total request body size to prevent OOM
	r.Body = http.MaxBytesReader(w, r.Body, shared.UploadMaxSize)
	if err := r.ParseMultipartForm(openAIUploadMaxMemory); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			shared.WriteOpenAIError(w, http.StatusRequestEntityTooLarge, "file size exceeds limit")
			return
		}
		shared.WriteOpenAIError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	if r.MultipartForm != nil {
		defer func() {
			if err := r.MultipartForm.RemoveAll(); err != nil {
				config.Logger.Warn("[files] failed to remove multipart temporary files", "error", err)
			}
		}()
	}
	r = r.WithContext(auth.WithAuth(r.Context(), a))
	file, header, err := r.FormFile("file")
	if err != nil {
		shared.WriteOpenAIError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			config.Logger.Warn("[files] failed to close uploaded file", "error", err)
		}
	}()
	data, err := io.ReadAll(file)
	if err != nil {
		shared.WriteOpenAIError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" && len(data) > 0 {
		contentType = http.DetectContentType(data)
	}
	modelType := resolveUploadModelType(h.Store, r)
	result, err := h.DS.UploadFile(r.Context(), a, dsclient.UploadFileRequest{
		Filename:        header.Filename,
		ContentType:     contentType,
		Purpose:         strings.TrimSpace(r.FormValue("purpose")),
		ModelType:       modelType,
		Data:            data,
		ThinkingEnabled: resolveUploadThinkingEnabled(r),
	}, 3)
	if err != nil {
		shared.WriteOpenAIError(w, http.StatusInternalServerError, "Failed to upload file.")
		return
	}
	if result == nil || strings.TrimSpace(result.ID) == "" {
		shared.WriteOpenAIError(w, http.StatusBadGateway, "DeepSeek returned no uploaded file ID.")
		return
	}
	a.RememberFileOwner(result.ID)
	if result.AccountID == "" {
		result.AccountID = a.AccountID
	}
	shared.WriteJSON(w, http.StatusOK, buildOpenAIFileObject(result))
}

func (h *Handler) RetrieveFile(w http.ResponseWriter, r *http.Request) {
	a, err := h.Auth.Determine(r)
	if err != nil {
		status := http.StatusUnauthorized
		detail := err.Error()
		if err == auth.ErrNoAccount {
			status = http.StatusTooManyRequests
		}
		shared.WriteOpenAIError(w, status, detail)
		return
	}
	defer h.Auth.Release(a)

	fileID := strings.TrimSpace(chi.URLParam(r, "file_id"))
	if fileID == "" {
		shared.WriteOpenAIError(w, http.StatusBadRequest, "file_id is required")
		return
	}
	files, err := inputfiles.ResolveFileReferences(r.Context(), a, h.DS, []string{fileID})
	if err != nil {
		status, message := inputfiles.MapError(err)
		shared.WriteOpenAIError(w, status, message)
		return
	}
	result := files[fileID]
	if result != nil && result.AccountID == "" {
		result.AccountID = a.AccountID
	}
	shared.WriteJSON(w, http.StatusOK, buildOpenAIFileObject(result))
}

func resolveUploadModelType(store shared.ConfigReader, r *http.Request) string {
	for _, candidate := range []string{r.FormValue("model_type"), r.Header.Get("X-Model-Type")} {
		if modelType := normalizeUploadModelType(candidate); modelType != "" {
			return modelType
		}
	}
	requestedModel := strings.TrimSpace(r.FormValue("model"))
	if requestedModel != "" {
		if resolvedModel, ok := config.ResolveModel(store, requestedModel); ok {
			if modelType, ok := config.GetModelType(resolvedModel); ok {
				return modelType
			}
		}
	}
	return "default"
}

func normalizeUploadModelType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "default", "expert", "vision":
		return "default"
	default:
		return ""
	}
}

func resolveUploadThinkingEnabled(r *http.Request) *bool {
	for _, candidate := range []string{r.FormValue("thinking_enabled"), r.Header.Get("X-Thinking-Enabled")} {
		var setting any = strings.TrimSpace(candidate)
		switch setting {
		case "1":
			setting = true
		case "0":
			setting = false
		}
		if enabled, ok := util.ResolveThinkingOverride(map[string]any{"thinking_enabled": setting}); ok {
			return &enabled
		}
	}
	return nil
}

func buildOpenAIFileObject(result *dsclient.UploadFileResult) map[string]any {
	if result == nil {
		obj := map[string]any{
			"id":             "",
			"object":         "file",
			"bytes":          0,
			"created_at":     time.Now().Unix(),
			"filename":       "",
			"purpose":        "",
			"status":         "uploaded",
			"status_details": nil,
		}
		return obj
	}
	obj := map[string]any{
		"id":             result.ID,
		"object":         "file",
		"bytes":          result.Bytes,
		"created_at":     time.Now().Unix(),
		"filename":       result.Filename,
		"purpose":        result.Purpose,
		"status":         result.Status,
		"status_details": nil,
	}
	if result.AccountID != "" {
		obj["account_id"] = result.AccountID
	}
	return obj
}
