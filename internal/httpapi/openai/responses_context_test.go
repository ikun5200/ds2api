package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestResponsesRejectsUnimplementedContinuationBeforeUploading(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{}, Auth: streamStatusAuthStub{}, DS: ds}
	router := chi.NewRouter()
	registerOpenAITestRoutes(router, h)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"deepseek-flash","previous_response_id":"resp_with_photo","input":[{"role":"user","content":[{"type":"input_text","text":"Compare with the previous photo"},{"type":"input_image","image_url":"data:image/png;base64,QUJDRA=="}]}]}`))
	req.Header.Set("Authorization", "Bearer direct-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "previous_response_id") {
		t.Fatalf("must explicitly reject unsupported context reference: %d %s", rec.Code, rec.Body.String())
	}
	if len(ds.uploadCalls) != 0 || ds.completionReq != nil {
		t.Fatal("unsupported continuation must not upload or generate without the earlier context")
	}
}
