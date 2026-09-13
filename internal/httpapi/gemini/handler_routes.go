package gemini

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/chathistory"
	"ds2api/internal/config"
	"ds2api/internal/textclean"
	"ds2api/internal/util"
)

var writeJSON = util.WriteJSON

type Handler struct {
	Store       ConfigReader
	Auth        AuthResolver
	DS          DeepSeekCaller
	OpenAI      OpenAIChatRunner
	ChatHistory *chathistory.Store
}

//nolint:unused // used by native Gemini stream/non-stream runtime helpers.
func stripReferenceMarkersEnabled() bool {
	return textclean.StripReferenceMarkersEnabled()
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Get("/v1beta/models", h.ListModels)
	r.Post("/v1beta/models/{model}:generateContent", h.GenerateContent)
	r.Post("/v1beta/models/{model}:streamGenerateContent", h.StreamGenerateContent)
	r.Post("/v1/models/{model}:generateContent", h.GenerateContent)
	r.Post("/v1/models/{model}:streamGenerateContent", h.StreamGenerateContent)
}

func (h *Handler) ListModels(w http.ResponseWriter, _ *http.Request) {
	models := make([]map[string]any, 0, len(config.DeepSeekModels))
	for _, model := range config.DeepSeekModels {
		models = append(models, map[string]any{
			"name":                       "models/" + model.ID,
			"baseModelId":                model.ID,
			"displayName":                "DeepSeek Flash",
			"supportedGenerationMethods": []string{"generateContent", "streamGenerateContent"},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (h *Handler) GenerateContent(w http.ResponseWriter, r *http.Request) {
	h.handleGenerateContent(w, r, false)
}

func (h *Handler) StreamGenerateContent(w http.ResponseWriter, r *http.Request) {
	h.handleGenerateContent(w, r, true)
}
