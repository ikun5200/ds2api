package claude

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ds2api/internal/inputfiles"
	"ds2api/internal/promptcompat"
)

func TestClaudeProxyPreservesAttachmentsForSharedUploader(t *testing.T) {
	for _, prepare := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			content string
			bytes   string
			refs    int
		}{
			{"image only", `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"QUJDRA=="}}]`, "ABCD", 2},
			{"top reference only", `[{"type":"text","text":"describe the uploaded image"}]`, "", 1},
			{"document only", `[{"type":"document","source":{"type":"base64","media_type":"text/plain","data":"aGVsbG8="}}]`, "hello", 2},
			{"file ID image", `[{"type":"image","source":{"type":"file","file_id":"file-image"}}]`, "", 2},
			{"tool document", `[{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"document","source":{"type":"text","media_type":"text/plain","data":"hello"}}]}]`, "hello", 2},
		} {
			t.Run(fmt.Sprintf("%s/prepare=%v", tc.name, prepare), func(t *testing.T) {
				openAI := &openAIProxyCaptureStub{}
				h := &Handler{Store: claudeHistoryConfig{}, OpenAI: openAI}
				path := "/anthropic/v1/messages"
				if prepare {
					path += "?__stream_prepare=1"
				}
				body := fmt.Sprintf(`{"model":"deepseek-flash","system":"keep this system","ref_file_ids":["file-existing"],"thinking_enabled":false,"search_enabled":true,"stream":%v,"messages":[{"role":"user","content":%s}]}`, prepare, tc.content)
				rec := httptest.NewRecorder()
				h.Messages(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
				if rec.Code != http.StatusOK {
					t.Fatalf("proxy failed: %d %s", rec.Code, rec.Body.String())
				}
				ds := &claudeCurrentInputDS{}
				if err := (&inputfiles.Service{DS: ds}).PreprocessInlineFileInputs(context.Background(), nil, openAI.seenReq); err != nil {
					t.Fatal(err)
				}
				if tc.bytes != "" && (len(ds.uploads) != 1 || string(ds.uploads[0].Data) != tc.bytes) {
					t.Fatalf("proxy lost attachment bytes: %#v", openAI.seenReq)
				}
				if refs := promptcompat.CollectOpenAIRefFileIDs(openAI.seenReq); len(refs) != tc.refs || refs[0] != "file-existing" {
					t.Fatalf("proxy lost file references: %v", refs)
				}
				standard, err := promptcompat.NormalizeOpenAIChatRequest(nil, openAI.seenReq, "")
				if err != nil || standard.Thinking || !standard.Search {
					t.Fatalf("shared normalization lost request modes: thinking=%v search=%v err=%v", standard.Thinking, standard.Search, err)
				}
				messages, _ := openAI.seenReq["messages"].([]any)
				if len(messages) < 2 || promptcompat.NormalizeOpenAIContentForPrompt(messages[0].(map[string]any)["content"]) != "keep this system" {
					t.Fatalf("system or attachment-only message lost: %#v", messages)
				}
			})
		}
	}
}
