package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestListModelsRouteOnlyFlash(t *testing.T) {
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
	if len(payload.Data) != 1 || payload.Data[0].ID != "deepseek-flash" {
		t.Fatalf("expected only deepseek-flash, got %s", rec.Body.String())
	}
}

func TestGetModelRouteDirectAndAlias(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	for _, model := range []string{
		"deepseek-flash", "deepseek-flash-nothinking",
		"deepseek-v4-flash", "deepseek-v4-flash-nothinking",
		"deepseek-v4-pro", "deepseek-v4-vision", "gpt-4.1",
		"claude-sonnet-4-6-nothinking",
	} {
		t.Run(model, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/models/"+model, nil)
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
			if payload.ID != "deepseek-flash" {
				t.Fatalf("expected canonical model ID, got %q", payload.ID)
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
