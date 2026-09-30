package ollama

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type ollamaTestSurface struct {
	Store   ConfigReader
	handler *Handler
}

func (h *ollamaTestSurface) apiHandler() *Handler {
	if h.handler == nil {
		h.handler = &Handler{Store: h.Store}
	}
	return h.handler
}

func registerOllamaTestRoutes(r chi.Router, h *ollamaTestSurface) {
	r.Get("/api/version", h.apiHandler().GetVersion)
	r.Get("/api/tags", h.apiHandler().ListOllamaModels)
	r.Post("/api/show", h.apiHandler().GetOllamaModel)
}

func TestGetOllamaVersionRoute(t *testing.T) {
	h := &ollamaTestSurface{}
	r := chi.NewRouter()
	registerOllamaTestRoutes(r, h)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetOllamaModelsRoute(t *testing.T) {
	h := &ollamaTestSurface{}
	r := chi.NewRouter()
	registerOllamaTestRoutes(r, h)
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode model list: %v", err)
	}
	if len(payload.Models) != 2 {
		t.Fatalf("expected flash and search models, got %s", rec.Body.String())
	}
	for i, want := range []string{"deepseek-flash", "deepseek-flash-search"} {
		if payload.Models[i].Name != want || payload.Models[i].Model != want {
			t.Fatalf("unexpected model %d: %#v", i, payload.Models[i])
		}
	}
}

func TestGetOllamaModelRoute(t *testing.T) {
	h := &ollamaTestSurface{}
	r := chi.NewRouter()
	registerOllamaTestRoutes(r, h)

	for _, model := range []string{"deepseek-flash", "deepseek-flash-search"} {
		t.Run(model, func(t *testing.T) {
			body := `{"model":"` + model + `"}`
			req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("expected valid json body, got err=%v body=%s", err, rec.Body.String())
			}
			if payload["id"] != model {
				t.Fatalf("expected model ID %q, body=%s", model, rec.Body.String())
			}
			capabilities, ok := payload["capabilities"].([]any)
			if !ok || len(capabilities) != 3 || capabilities[0] != "tools" || capabilities[1] != "thinking" || capabilities[2] != "vision" {
				t.Fatalf("unexpected flash capabilities: %#v", payload["capabilities"])
			}
			if _, ok := payload["ID"]; ok {
				t.Fatalf("expected response does not expose uppercase ID field, body=%s", rec.Body.String())
			}
		})
	}

	t.Run("direct_nothinking", func(t *testing.T) {
		body := `{"model":"deepseek-v4-flash-nothinking"}`
		req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("direct_expert", func(t *testing.T) {
		body := `{"model":"deepseek-v4-pro"}`
		req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("direct_vision", func(t *testing.T) {
		body := `{"model":"deepseek-v4-vision"}`
		req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestGetOllamaModelRouteNotFound(t *testing.T) {
	h := &ollamaTestSurface{}
	r := chi.NewRouter()
	registerOllamaTestRoutes(r, h)

	body := `{"model":"not-exists"}`
	req := httptest.NewRequest(http.MethodPost, "/api/show", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
