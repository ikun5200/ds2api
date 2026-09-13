package inputfiles

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type uploadStub struct{ requests []dsclient.UploadFileRequest }

func (s *uploadStub) UploadFile(_ context.Context, _ *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	s.requests = append(s.requests, req)
	return &dsclient.UploadFileResult{ID: fmt.Sprintf("file-%d", len(s.requests)), TokenUsage: 117}, nil
}

func (s *uploadStub) FetchUploadedFile(_ context.Context, _ *auth.RequestAuth, id string) (*dsclient.UploadFileResult, error) {
	return &dsclient.UploadFileResult{ID: id, Status: "SUCCESS"}, nil
}

type attachmentRoundTripper func(*http.Request) (*http.Response, error)

func (f attachmentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestApplyUsesActualTokensAndDeduplicatesNormalizedAttachments(t *testing.T) {
	stub := &uploadStub{}
	service := &Service{DS: stub}
	image := map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}
	got, err := service.Apply(context.Background(), &auth.RequestAuth{}, promptcompat.StandardRequest{
		ResolvedModel: "deepseek-flash", RefFileIDs: []string{"existing"},
		Messages: []any{map[string]any{"role": "user", "content": "describe", "attachments": []any{image, image}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stub.requests) != 1 || got.RefFileTokens != 117 || len(got.RefFileIDs) != 2 || got.RefFileIDs[1] != "file-1" {
		t.Fatalf("unexpected deduplication or actual token accounting: uploads=%d tokens=%d ids=%v", len(stub.requests), got.RefFileTokens, got.RefFileIDs)
	}
	message := got.Messages[0].(map[string]any)
	block := message["attachments"].([]any)[0].(map[string]any)
	if block["file_id"] != "file-1" || block["image_url"] != nil {
		t.Fatalf("binary data not replaced with reference: %#v", block)
	}
}

func TestRemoteAttachmentsDownloadAndUpload(t *testing.T) {
	oldClient := attachmentHTTPClient
	t.Cleanup(func() { attachmentHTTPClient = oldClient })
	attachmentHTTPClient = &http.Client{Transport: attachmentRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://example.com/photo.png" {
			t.Fatalf("unexpected URL: %s", req.URL)
		}
		if req.Header.Get("Authorization") != "" {
			t.Fatal("upstream credentials must not be forwarded")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(strings.NewReader("image bytes"))}, nil
	})}
	stub := &uploadStub{}
	request := map[string]any{"model": "deepseek-flash", "input": []any{map[string]any{"type": "input_image", "image_url": "https://example.com/photo.png"}}}
	if err := (&Service{DS: stub}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{DeepSeekToken: "private-token"}, request); err != nil {
		t.Fatal(err)
	}
	if len(stub.requests) != 1 || string(stub.requests[0].Data) != "image bytes" || stub.requests[0].Filename != "photo.png" {
		t.Fatalf("unexpected downloaded attachment: %#v", stub.requests)
	}
}

func TestAttachmentURLsRejectInternalAndNonHTTPDestinations(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com/file", "http://127.0.0.1/a", "http://[::1]/a", "http://169.254.169.254/a", "http://10.0.0.1/a", "http://100.100.100.200/a", "http://user:password@example.com/a"} {
		t.Run(raw, func(t *testing.T) {
			u, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if validateAttachmentURL(u) == nil {
				t.Fatalf("unsafe URL accepted: %s", raw)
			}
		})
	}
	for _, raw := range []string{"127.0.0.1", "::ffff:127.0.0.1", "10.0.0.1", "169.254.169.254", "100.100.100.200", "fc00::1", "fe80::1"} {
		if isPublicAttachmentIP(net.ParseIP(raw)) {
			t.Fatalf("unsafe resolved IP accepted: %s", raw)
		}
	}
	if !isPublicAttachmentIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public IP rejected")
	}
	if _, err := dialPublicAttachment(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("dial must enforce the public IP policy")
	}
}

func TestRemoteAttachmentRejectsExcessiveSize(t *testing.T) {
	oldClient := attachmentHTTPClient
	t.Cleanup(func() { attachmentHTTPClient = oldClient })
	attachmentHTTPClient = &http.Client{Transport: attachmentRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: maxAttachmentBytes + 1, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("unused"))}, nil
	})}
	stub := &uploadStub{}
	request := map[string]any{"input": []any{map[string]any{"type": "input_file", "file_url": "https://example.com/document.pdf"}}}
	err := (&Service{DS: stub}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, request)
	status, _ := MapError(err)
	if err == nil || status != http.StatusBadRequest || len(stub.requests) != 0 {
		t.Fatalf("oversized remote file must be rejected before upload: %v", err)
	}
}

type accountUploadStub struct {
	changeAccount bool
	calls         int
}

func (s *accountUploadStub) FetchUploadedFile(_ context.Context, a *auth.RequestAuth, id string) (*dsclient.UploadFileResult, error) {
	if a.AccountID != "account-one" {
		return nil, fmt.Errorf("file reference verification changed the selected account")
	}
	return &dsclient.UploadFileResult{ID: id, Status: "SUCCESS", AccountID: a.AccountID}, nil
}

func (s *accountUploadStub) UploadFile(_ context.Context, a *auth.RequestAuth, _ dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	s.calls++
	if a.TargetAccount != "account-one" {
		return nil, fmt.Errorf("attachment account was not pinned")
	}
	if s.changeAccount {
		a.AccountID = "account-two"
	}
	return &dsclient.UploadFileResult{ID: fmt.Sprintf("uploaded-file-%d", s.calls)}, nil
}

func TestAttachmentRequestsPinTheSelectedAccount(t *testing.T) {
	for _, inline := range []bool{false, true} {
		t.Run(fmt.Sprint("inline=", inline), func(t *testing.T) {
			a := &auth.RequestAuth{UseConfigToken: true, AccountID: "account-one"}
			uploader := &accountUploadStub{}
			req := map[string]any{"ref_file_ids": []any{"existing-file"}}
			if inline {
				req = map[string]any{"input": []any{
					map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="},
					map[string]any{"type": "input_image", "image_url": "data:image/png;base64,aGVsbG8="},
				}}
			}
			if err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), a, req); err != nil {
				t.Fatal(err)
			}
			if a.TargetAccount != "account-one" {
				t.Fatalf("file references must pin subsequent PoW/session/completion requests, got %#v", a)
			}
			if inline && uploader.calls != 2 {
				t.Fatalf("expected both attachments to be uploaded on the pinned account: %d", uploader.calls)
			}
			if (&auth.Resolver{}).SwitchAccount(context.Background(), a) {
				t.Fatal("subsequent request retries must retain the file account")
			}
		})
	}
}

func TestAttachmentRequestsRejectUnexpectedAccountChange(t *testing.T) {
	a := &auth.RequestAuth{UseConfigToken: true, AccountID: "account-one"}
	req := map[string]any{"input": []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}}}
	err := (&Service{DS: &accountUploadStub{changeAccount: true}}).PreprocessInlineFileInputs(context.Background(), a, req)
	status, message := MapError(err)
	if err == nil || status != http.StatusInternalServerError || !strings.Contains(message, "same DeepSeek account") {
		t.Fatalf("must reject references from a different account: %v", err)
	}
}

func TestStandardAttachmentsUseResolvedThinkingMode(t *testing.T) {
	for _, thinking := range []bool{false, true} {
		t.Run(fmt.Sprint(thinking), func(t *testing.T) {
			stub := &uploadStub{}
			_, err := (&Service{DS: stub}).Apply(context.Background(), &auth.RequestAuth{}, promptcompat.StandardRequest{
				ResolvedModel: "deepseek-flash", Thinking: thinking,
				Messages: []any{map[string]any{"role": "user", "attachments": []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			got := stub.requests[0].ThinkingEnabled
			if got == nil || *got != thinking {
				t.Fatalf("attachment upload did not inherit resolved thinking=%v: %v", thinking, got)
			}
		})
	}
}

func TestInlineAttachmentsShareRequestThinkingResolution(t *testing.T) {
	for _, tc := range []struct {
		name     string
		settings map[string]any
		want     bool
	}{
		{name: "default", settings: map[string]any{}, want: true},
		{name: "explicit false", settings: map[string]any{"thinking_enabled": false}, want: false},
		{name: "thinking disabled", settings: map[string]any{"thinking": map[string]any{"type": "disabled"}}, want: false},
		{name: "reasoning effort", settings: map[string]any{"reasoning_effort": "none"}, want: false},
		{name: "legacy nothinking", settings: map[string]any{"model": "deepseek-v4-flash-nothinking", "thinking_enabled": true}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &uploadStub{}
			req := map[string]any{"model": "deepseek-flash", "input": []any{map[string]any{"type": "input_file", "file_data": "aGVsbG8="}}}
			for key, value := range tc.settings {
				req[key] = value
			}
			if err := (&Service{DS: stub}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req); err != nil {
				t.Fatal(err)
			}
			got := stub.requests[0].ThinkingEnabled
			if got == nil || *got != tc.want {
				t.Fatalf("unexpected resolved upload thinking: got=%v want=%v", got, tc.want)
			}
		})
	}
}
