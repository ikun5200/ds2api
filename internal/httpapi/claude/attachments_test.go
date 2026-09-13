package claude

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeDirectUploadsImagesAndDocuments(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint("stream=", stream), func(t *testing.T) {
			ds := &claudeCurrentInputDS{}
			h := &Handler{Store: claudeHistoryConfig{}, Auth: claudeCurrentInputAuth{}, DS: ds}
			body := fmt.Sprintf(`{"model":"deepseek-flash","stream":%t,"messages":[{"role":"user","content":[{"type":"text","text":"compare these attachments"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJDRA=="}},{"type":"document","title":"notes.txt","source":{"type":"base64","media_type":"text/plain","data":"aGVsbG8="}},{"type":"document","source":{"type":"file","file_id":"file-existing"}}]}]}`, stream)
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
			rec := httptest.NewRecorder()
			h.Messages(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("unexpected response: %d %s", rec.Code, rec.Body.String())
			}
			if len(ds.uploads) != 2 || string(ds.uploads[0].Data) != "ABCD" || string(ds.uploads[1].Data) != "hello" {
				t.Fatalf("expected both native attachments to be uploaded, got %#v", ds.uploads)
			}
			for _, upload := range ds.uploads {
				if upload.ModelType != "default" {
					t.Fatalf("attachment must use Flash default model type: %#v", upload)
				}
			}
			refs, _ := ds.payload["ref_file_ids"].([]any)
			if len(refs) != 3 || refs[0] != "file-existing" || refs[1] != "file-claude-history" || refs[2] != "file-claude-tools" {
				t.Fatalf("unexpected attachment references: %#v", refs)
			}
			prompt, _ := ds.payload["prompt"].(string)
			if strings.Contains(prompt, "QUJDRA==") || strings.Contains(prompt, "aGVsbG8=") {
				t.Fatal("binary data must not enter the text prompt")
			}
		})
	}
}

func TestClaudeDirectRejectsMalformedImageBeforeCompletion(t *testing.T) {
	ds := &claudeCurrentInputDS{}
	h := &Handler{Store: claudeHistoryConfig{}, Auth: claudeCurrentInputAuth{}, DS: ds}
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"deepseek-flash","messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"%%%"}}]}]}`))
	rec := httptest.NewRecorder()
	h.Messages(rec, req)
	if rec.Code != http.StatusBadRequest || ds.payload != nil || len(ds.uploads) != 0 {
		t.Fatalf("malformed image must fail before upload/completion: %d %s", rec.Code, rec.Body.String())
	}
}
