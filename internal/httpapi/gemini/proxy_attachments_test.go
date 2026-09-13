package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/inputfiles"
	"ds2api/internal/promptcompat"
)

func TestGeminiProxyPreservesNativeAttachmentsForSharedUploader(t *testing.T) {
	for _, prepare := range []bool{false, true} {
		for _, tc := range []struct {
			name, part, bytes string
			refs              int
		}{
			{"snake image only", `{"inline_data":{"mime_type":"image/png","data":"QUJDRA=="}}`, "ABCD", 2},
			{"tool image", `{"functionResponse":{"name":"capture","response":{"result":"done"},"parts":[{"inlineData":{"mimeType":"image/png","data":"QUJDRA=="}}]}}`, "ABCD", 2},
			{"document only", `{"inlineData":{"mimeType":"text/plain","data":"aGVsbG8="}}`, "hello", 2},
			{"file ID image", `{"fileData":{"mimeType":"image/png","file_id":"file-image"}}`, "", 2},
			{"top reference only", `{"text":"describe the uploaded image"}`, "", 1},
		} {
			t.Run(fmt.Sprintf("%s/prepare=%v", tc.name, prepare), func(t *testing.T) {
				openAI := &geminiOpenAISuccessStub{}
				h := &Handler{Store: attachmentGeminiConfig{}, OpenAI: openAI}
				router := chi.NewRouter()
				RegisterRoutes(router, h)
				path := "/v1beta/models/deepseek-flash:generateContent"
				if prepare {
					path = "/v1beta/models/deepseek-flash:streamGenerateContent?__stream_prepare=1"
				}
				body := `{"ref_file_ids":["file-existing"],"generation_config":{"thinking_config":{"thinking_budget":0}},"tools":[{"google_search":{}}],"contents":[{"role":"user","parts":[` + tc.part + `]}]}`
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
				if rec.Code != http.StatusOK {
					t.Fatalf("proxy failed: %d %s", rec.Code, rec.Body.String())
				}
				ds := &testGeminiDS{}
				if err := (&inputfiles.Service{DS: ds}).PreprocessInlineFileInputs(context.Background(), nil, openAI.seenReq); err != nil {
					t.Fatal(err)
				}
				if tc.bytes != "" && (len(ds.uploadCalls) != 1 || string(ds.uploadCalls[0].Data) != tc.bytes) {
					t.Fatalf("proxy lost image bytes: %#v", openAI.seenReq)
				}
				if refs := promptcompat.CollectOpenAIRefFileIDs(openAI.seenReq); len(refs) != tc.refs || refs[0] != "file-existing" {
					t.Fatalf("proxy lost file references: %v", refs)
				}
				standard, err := promptcompat.NormalizeOpenAIChatRequest(nil, openAI.seenReq, "")
				if err != nil || standard.Thinking || !standard.Search {
					t.Fatalf("shared normalization lost request modes: thinking=%v search=%v err=%v", standard.Thinking, standard.Search, err)
				}
			})
		}
	}
}

func TestGeminiDirectUploadsFunctionResponseImage(t *testing.T) {
	ds := &testGeminiDS{resp: makeGeminiUpstreamResponse(`data: {"p":"response/content","v":"ok"}`)}
	h := &Handler{Store: attachmentGeminiConfig{}, Auth: testGeminiAuth{}, DS: ds}
	router := chi.NewRouter()
	RegisterRoutes(router, h)
	body := `{"contents":[{"role":"user","parts":[{"functionResponse":{"name":"capture","response":{"result":"done"},"parts":[{"inlineData":{"mimeType":"image/png","data":"QUJDRA=="}}]}}]}]}`
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1beta/models/deepseek-flash:generateContent", strings.NewReader(body)))
	if rec.Code != http.StatusOK || len(ds.uploadCalls) != 1 || string(ds.uploadCalls[0].Data) != "ABCD" {
		t.Fatalf("tool response image was not uploaded: status=%d uploads=%#v", rec.Code, ds.uploadCalls)
	}
}
