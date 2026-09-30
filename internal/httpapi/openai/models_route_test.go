package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestListModelsRouteFlashVariants(t *testing.T) {
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, &openAITestSurface{})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode model list: %v", err)
	}
	if len(payload.Data) != 2 || payload.Data[0].ID != "deepseek-flash" || payload.Data[1].ID != "deepseek-flash-search" {
		t.Fatalf("expected flash and search models, got %s", rec.Body.String())
	}
}

func TestGetModelRouteDirectAndAlias(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	for _, tc := range []struct{ model, want string }{
		{"deepseek-flash", "deepseek-flash"},
		{"deepseek-flash-nothinking", "deepseek-flash"},
		{"deepseek-flash-search", "deepseek-flash-search"},
		{"deepseek-flash-search-nothinking", "deepseek-flash-search"},
		{"deepseek-v4-flash", "deepseek-flash"},
		{"deepseek-v4-flash-nothinking", "deepseek-flash"},
		{"deepseek-v4-pro", "deepseek-flash"},
		{"deepseek-v4-vision", "deepseek-flash"},
		{"deepseek-v4-pro-search", "deepseek-flash-search"},
		{"gpt-4.1", "deepseek-flash"},
		{"o3-deep-research", "deepseek-flash-search"},
		{"gpt-4.1-nothinking", "deepseek-flash"},
	} {
		t.Run(tc.model, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models/"+tc.model, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var payload struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode model: %v", err)
			}
			if payload.ID != tc.want {
				t.Fatalf("expected canonical model ID %q, got %q", tc.want, payload.ID)
			}
		})
	}
}

func TestGetModelRouteNotFound(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/not-exists", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
