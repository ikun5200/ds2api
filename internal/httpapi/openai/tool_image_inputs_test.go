package openai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestResponsesToolOutputImagesReachDeepSeek(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			image    map[string]any
			wantFile string
			uploads  int
		}{
			{"inline screenshot", map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}, "file-inline-1", 1},
			{"existing screenshot", map[string]any{"type": "input_image", "file_id": "file-screenshot"}, "file-screenshot", 0},
			{"image_file screenshot", map[string]any{"type": "image_file", "image_file": map[string]any{"file_id": "file-screenshot"}}, "file-screenshot", 0},
		} {
			t.Run(tc.name+"/stream="+map[bool]string{true: "true", false: "false"}[stream], func(t *testing.T) {
				ds := &inlineUploadDSStub{}
				h := &openAITestSurface{Store: mockOpenAIConfig{}, Auth: streamStatusAuthStub{}, DS: ds}
				router := chi.NewRouter()
				registerOpenAITestRoutes(router, h)
				body, err := json.Marshal(map[string]any{"model": "deepseek-flash", "stream": stream, "thinking_enabled": false, "input": []any{
					map[string]any{"type": "function_call", "name": "screenshot", "call_id": "call-1", "arguments": "{}"},
					map[string]any{"type": "function_call_output", "call_id": "call-1", "output": []any{map[string]any{"type": "input_text", "text": "Screenshot captured"}, tc.image}},
					map[string]any{"role": "user", "content": "Describe the screenshot"},
				}})
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(body)))
				req.Header.Set("Authorization", "Bearer direct-token")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
				}
				if len(ds.uploadCalls) != tc.uploads {
					t.Fatalf("uploads=%d, want %d", len(ds.uploadCalls), tc.uploads)
				}
				refs, _ := ds.completionReq["ref_file_ids"].([]any)
				if len(refs) != 1 || refs[0] != tc.wantFile {
					t.Fatalf("screenshot missing from DeepSeek request: %#v", ds.completionReq)
				}
				prompt, _ := ds.completionReq["prompt"].(string)
				if strings.Contains(prompt, "QUJDRA==") {
					t.Fatal("binary screenshot leaked into text prompt")
				}
			})
		}
	}
}
