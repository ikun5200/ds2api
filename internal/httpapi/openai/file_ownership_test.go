package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/account"
	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/httpapi/claude"
	"ds2api/internal/httpapi/gemini"
)

type fileOwnerResolver struct {
	*auth.Resolver
	selected []string
}

func (r *fileOwnerResolver) Determine(req *http.Request) (*auth.RequestAuth, error) {
	a, err := r.Resolver.Determine(req)
	if err == nil {
		r.selected = append(r.selected, a.AccountID)
	}
	return a, err
}

type fileOwnerDS struct {
	filesRouteDSStub
	owners    map[string]string
	status    string
	completed []string
	verified  []string
}

func (d *fileOwnerDS) UploadFile(_ context.Context, a *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	id := "file-" + strings.Split(a.AccountID, "@")[0]
	d.owners[id] = a.AccountID
	return &dsclient.UploadFileResult{ID: id, AccountID: a.AccountID, Status: "SUCCESS", Filename: req.Filename}, nil
}

func (d *fileOwnerDS) FetchUploadedFile(_ context.Context, a *auth.RequestAuth, id string) (*dsclient.UploadFileResult, error) {
	d.verified = append(d.verified, a.AccountID)
	if owner := d.owners[id]; owner == "" || owner != a.AccountID {
		return nil, dsclient.ErrUploadFileNotFound
	}
	status := d.status
	if status == "" {
		status = "SUCCESS"
	}
	return &dsclient.UploadFileResult{ID: id, Status: status}, nil
}

func (d *fileOwnerDS) CreateSession(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "session", nil
}

func (d *fileOwnerDS) GetPow(_ context.Context, _ *auth.RequestAuth, _ int) (string, error) {
	return "pow", nil
}

func (d *fileOwnerDS) CallCompletion(_ context.Context, a *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	refs, _ := payload["ref_file_ids"].([]any)
	for _, raw := range refs {
		if d.owners[fmt.Sprint(raw)] != a.AccountID {
			return nil, fmt.Errorf("file sent to wrong account")
		}
	}
	d.completed = append(d.completed, a.AccountID)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("data: {\"p\":\"response/content\",\"v\":\"image received\"}\ndata: [DONE]\n"))}, nil
}

func fileOwnershipRouter(t *testing.T) (chi.Router, *fileOwnerResolver, *fileOwnerDS) {
	t.Helper()
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["managed-key"],"accounts":[{"email":"a@example.com","password":"p"},{"email":"b@example.com","password":"p"}],"current_input_file":{"enabled":false},"runtime":{"account_max_inflight":1,"global_max_inflight":1}}`)
	t.Setenv("DS2API_ENV_WRITEBACK", "0")
	store := config.LoadStore()
	resolver := &fileOwnerResolver{Resolver: auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) {
		return "token-" + acc.Identifier(), nil
	})}
	ds := &fileOwnerDS{owners: map[string]string{}}
	h := &openAITestSurface{Store: store, Auth: resolver, DS: ds}
	router := chi.NewRouter()
	registerOpenAITestRoutes(router, h)
	claude.RegisterRoutes(router, &claude.Handler{Store: store, Auth: resolver, DS: ds})
	gemini.RegisterRoutes(router, &gemini.Handler{Store: store, Auth: resolver, DS: ds})
	return router, resolver, ds
}

func uploadOwnedTestFile(t *testing.T, router http.Handler, target string) {
	t.Helper()
	req := newMultipartUploadRequest(t, "assistants", "image.png", []byte("image"), "deepseek-flash")
	req.Header.Set("Authorization", "Bearer managed-key")
	if target != "" {
		req.Header.Set("X-Ds2-Target-Account", target)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
}

func TestUploadedFileFollowsOwningAccountAcrossProtocols(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"describe"}],"file_ids":["file-a"]}`},
		{"/v1/responses", `{"model":"deepseek-flash","input":"describe","file_ids":["file-a"]}`},
		{"/anthropic/v1/messages", `{"model":"deepseek-flash","messages":[{"role":"user","content":[{"type":"image","source":{"type":"file","file_id":"file-a"}}]}]}`},
		{"/v1beta/models/deepseek-flash:generateContent", `{"contents":[{"role":"user","parts":[{"text":"describe"}]}],"ref_file_ids":["file-a"]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			router, resolver, ds := fileOwnershipRouter(t)
			uploadOwnedTestFile(t, router, "")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body)).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer managed-key")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || len(ds.completed) != 1 || ds.completed[0] != "a@example.com" {
				t.Fatalf("generation did not use file owner: %d %s completed=%v", rec.Code, rec.Body.String(), ds.completed)
			}
			if len(resolver.selected) < 2 || resolver.selected[1] != "b@example.com" {
				t.Fatalf("test must start generation with account B: %v", resolver.selected)
			}
			get := httptest.NewRequest(http.MethodGet, "/v1/files/file-a", nil)
			get.Header.Set("Authorization", "Bearer managed-key")
			got := httptest.NewRecorder()
			router.ServeHTTP(got, get)
			var meta map[string]any
			if err := json.Unmarshal(got.Body.Bytes(), &meta); err != nil {
				t.Fatal(err)
			}
			if got.Code != http.StatusOK || meta["account_id"] != "a@example.com" || resolver.Pool.Status()["in_use"] != 0 {
				t.Fatalf("retrieval owner/lease mismatch: %d %s pool=%v", got.Code, got.Body.String(), resolver.Pool.Status())
			}
		})
	}
}

func TestFileOwnerConflictsStopBeforeCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, refs, target string
		status             int
	}{
		{"explicit target", `["file-a"]`, "b@example.com", http.StatusConflict},
		{"mixed owners", `["file-a","file-b"]`, "", http.StatusConflict},
		{"unknown file", `["not-found"]`, "", http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router, resolver, ds := fileOwnershipRouter(t)
			uploadOwnedTestFile(t, router, "a@example.com")
			if tc.name == "mixed owners" {
				uploadOwnedTestFile(t, router, "b@example.com")
			}
			body := `{"model":"deepseek-flash","messages":[{"role":"user","content":"describe"}],"file_ids":` + tc.refs + `}`
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer managed-key")
			req.Header.Set("X-Ds2-Target-Account", tc.target)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status || len(ds.completed) != 0 || resolver.Pool.Status()["in_use"] != 0 {
				t.Fatalf("unsafe reference request proceeded: %d %s completed=%v", rec.Code, rec.Body.String(), ds.completed)
			}
			if tc.status == http.StatusNotFound && !strings.Contains(rec.Body.String(), "X-Ds2-Target-Account") {
				t.Fatalf("unknown-file error lacks recovery guidance: %s", rec.Body.String())
			}
		})
	}
}

func TestPendingFileCanBeRetrievedButCannotGenerate(t *testing.T) {
	router, _, ds := fileOwnershipRouter(t)
	uploadOwnedTestFile(t, router, "")
	ds.status = "PARSING"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path, body, want := "/v1/files/file-a", "", http.StatusOK
		if method == http.MethodPost {
			path, body, want = "/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"describe"}],"file_ids":["file-a"]}`, http.StatusConflict
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer managed-key")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != want || len(ds.completed) != 0 {
			t.Fatalf("pending-file status mismatch: %d %s", rec.Code, rec.Body.String())
		}
	}
}

func TestUncachedReferenceValidatesCurrentCredential(t *testing.T) {
	router, resolver, ds := fileOwnershipRouter(t)
	// Simulate a file uploaded on another instance: DeepSeek knows its owner,
	// while this resolver has no local ownership record.
	ds.owners["file-a"] = "a@example.com"
	warmup := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	warmup.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(warmup)
	if err != nil {
		t.Fatal(err)
	}
	resolver.Release(a)
	body := `{"model":"deepseek-flash","messages":[{"role":"user","content":"describe"}],"file_ids":["file-a"]}`
	for _, tc := range []struct {
		token, target string
		status        int
	}{
		{"managed-key", "", http.StatusNotFound},
		{"managed-key", "a@example.com", http.StatusOK},
		{"unrelated-direct-token", "", http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tc.token)
		req.Header.Set("X-Ds2-Target-Account", tc.target)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("unknown-owner credential handling: status=%d want=%d body=%s", rec.Code, tc.status, rec.Body.String())
		}
	}
	if len(ds.completed) != 1 || ds.completed[0] != "a@example.com" || resolver.Pool.Status()["in_use"] != 0 {
		t.Fatalf("unknown ownership silently proceeded: completed=%v pool=%v", ds.completed, resolver.Pool.Status())
	}
}

type emptyUploadResultDS struct {
	filesRouteDSStub
	result *dsclient.UploadFileResult
}

func (d *emptyUploadResultDS) UploadFile(context.Context, *auth.RequestAuth, dsclient.UploadFileRequest, int) (*dsclient.UploadFileResult, error) {
	return d.result, nil
}

func TestUploadWithoutFileIDFailsClearly(t *testing.T) {
	for _, result := range []*dsclient.UploadFileResult{nil, {ID: " "}} {
		h := &openAITestSurface{Store: mockOpenAIConfig{}, Auth: streamStatusAuthStub{}, DS: &emptyUploadResultDS{result: result}}
		router := chi.NewRouter()
		registerOpenAITestRoutes(router, h)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, newMultipartUploadRequest(t, "assistants", "image.png", []byte("image"), "deepseek-flash"))
		if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "no uploaded file ID") {
			t.Fatalf("empty upload incorrectly succeeded: %d %s", rec.Code, rec.Body.String())
		}
	}
}
