package gemini

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type attachmentGeminiConfig struct{}

func (attachmentGeminiConfig) ModelAliases() map[string]string { return nil }
func (attachmentGeminiConfig) CurrentInputFileEnabled() bool   { return false }
func (attachmentGeminiConfig) CurrentInputFileMinChars() int   { return 0 }

func TestGeminiDirectUploadsImagesAndDocuments(t *testing.T) {
	for _, method := range []string{"generateContent", "streamGenerateContent"} {
		t.Run(method, func(t *testing.T) {
			ds := &testGeminiDS{resp: makeGeminiUpstreamResponse(`data: {"p":"response/content","v":"ok"}`, `data: [DONE]`)}
			h := &Handler{Store: attachmentGeminiConfig{}, Auth: testGeminiAuth{}, DS: ds}
			router := chi.NewRouter()
			RegisterRoutes(router, h)
			body := `{"ref_file_ids":["file-existing"],"contents":[{"role":"user","parts":[{"text":"compare these attachments"},{"inlineData":{"mimeType":"image/png","data":"QUJDRA=="}},{"inline_data":{"mime_type":"text/plain","data":"aGVsbG8="}}]}]}`
			req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-flash:"+method, strings.NewReader(body))
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
			}
			if len(ds.uploadCalls) != 2 || string(ds.uploadCalls[0].Data) != "ABCD" || string(ds.uploadCalls[1].Data) != "hello" {
				t.Fatalf("expected both native attachments to be uploaded, got %#v", ds.uploadCalls)
			}
			refs, _ := ds.payloads[0]["ref_file_ids"].([]any)
			if len(refs) != 3 || refs[0] != "file-existing" || refs[1] != "file-gemini-history" || refs[2] != "file-gemini-tools" {
				t.Fatalf("unexpected attachment references: %#v", refs)
			}
			prompt, _ := ds.payloads[0]["prompt"].(string)
			if strings.Contains(prompt, "QUJDRA==") || strings.Contains(prompt, "aGVsbG8=") {
				t.Fatal("binary data must not enter the text prompt")
			}
		})
	}
}

func TestGeminiToolResultsAreNotDecodedAsAttachments(t *testing.T) {
	for _, method := range []string{"generateContent", "streamGenerateContent"} {
		for _, tc := range []struct {
			name     string
			response string
			want     string
		}{
			{name: "plain data", response: `{"name":"Beijing","data":"Sunny today"}`, want: "Sunny today"},
			{name: "base64-looking data", response: `{"name":"Beijing","data":"c3Vubnk="}`, want: "c3Vubnk="},
			{name: "ordinary typed data", response: `{"type":"profile","data":"Sunny today"}`, want: "Sunny today"},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				ds := &testGeminiDS{resp: makeGeminiUpstreamResponse(`data: {"p":"response/content","v":"ok"}`, `data: [DONE]`)}
				h := &Handler{Store: attachmentGeminiConfig{}, Auth: testGeminiAuth{}, DS: ds}
				router := chi.NewRouter()
				RegisterRoutes(router, h)
				body := `{"contents":[{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"Beijing"}}}]},{"role":"user","parts":[{"functionResponse":{"name":"get_weather","response":` + tc.response + `}}]},{"role":"user","parts":[{"text":"Summarize the weather"}]}]}`
				req := httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-flash:"+method, strings.NewReader(body))
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("ordinary tool result rejected: %d %s", rec.Code, rec.Body.String())
				}
				if len(ds.uploadCalls) != 0 || len(ds.payloads) != 1 {
					t.Fatalf("tool result triggered attachment processing: uploads=%d completions=%d", len(ds.uploadCalls), len(ds.payloads))
				}
				prompt, _ := ds.payloads[0]["prompt"].(string)
				if !strings.Contains(prompt, tc.want) {
					t.Fatalf("tool result content was lost from prompt: %s", prompt)
				}
				if refs, _ := ds.payloads[0]["ref_file_ids"].([]any); len(refs) != 0 {
					t.Fatalf("tool data became a file reference: %#v", refs)
				}
			})
		}
	}
}
