package gemini

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestListModelsOnlyFlash(t *testing.T) {
	r := chi.NewRouter()
	RegisterRoutes(r, &Handler{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []struct {
			Name                       string   `json:"name"`
			BaseModelID                string   `json:"baseModelId"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode Gemini model list: %v", err)
	}
	if len(payload.Models) != 1 || payload.Models[0].Name != "models/deepseek-flash" || payload.Models[0].BaseModelID != "deepseek-flash" {
		t.Fatalf("expected only deepseek-flash, got %s", rec.Body.String())
	}
	methods := payload.Models[0].SupportedGenerationMethods
	if len(methods) != 2 || methods[0] != "generateContent" || methods[1] != "streamGenerateContent" {
		t.Fatalf("unexpected supported generation methods: %v", methods)
	}
}
