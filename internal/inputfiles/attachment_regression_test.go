package inputfiles

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/auth"
)

func TestToolOutputAttachmentsReachTheUploader(t *testing.T) {
	for _, typ := range []string{"function_call_output", "tool_result"} {
		t.Run(typ, func(t *testing.T) {
			uploader := &uploadStub{}
			req := map[string]any{"input": []any{map[string]any{
				"type": typ, "output": []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}},
			}}}
			if err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req); err != nil {
				t.Fatal(err)
			}
			if len(uploader.requests) != 1 {
				t.Fatalf("tool output image silently skipped: uploads=%d", len(uploader.requests))
			}
			item := req["input"].([]any)[0].(map[string]any)
			block := item["output"].([]any)[0].(map[string]any)
			if block["file_id"] != "file-1" || block["image_url"] != nil {
				t.Fatalf("tool output binary was not replaced by an uploaded reference: %#v", block)
			}
		})
	}
}

func TestBusinessOutputFieldDoesNotBecomeAnAttachmentContainer(t *testing.T) {
	uploader := &uploadStub{}
	req := map[string]any{"messages": []any{map[string]any{
		"role": "tool", "content": map[string]any{"output": []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}}},
	}}}
	if err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req); err != nil {
		t.Fatal(err)
	}
	if len(uploader.requests) != 0 {
		t.Fatal("business JSON output field must not trigger upload")
	}
}

func TestNestedFileReferenceTakesPrecedenceOverDownloadSource(t *testing.T) {
	oldClient := attachmentHTTPClient
	t.Cleanup(func() { attachmentHTTPClient = oldClient })
	downloads := 0
	attachmentHTTPClient = &http.Client{Transport: attachmentRoundTripper(func(*http.Request) (*http.Response, error) {
		downloads++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("file bytes"))}, nil
	})}
	for _, key := range []string{"file_id", "id"} {
		t.Run(key, func(t *testing.T) {
			uploader := &uploadStub{}
			req := map[string]any{"input": []any{map[string]any{
				"type": "file", "file": map[string]any{key: "existing-file", "file_url": "https://example.com/photo.png"},
			}}}
			if err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req); err != nil {
				t.Fatal(err)
			}
			if len(uploader.requests) != 0 || downloads != 0 {
				t.Fatalf("existing nested file reference was replaced by a download: uploads=%d downloads=%d", len(uploader.requests), downloads)
			}
		})
	}
}

func TestMalformedImageBlockIsRejectedInsteadOfSilentlyDropped(t *testing.T) {
	for _, typ := range []string{"input_image", "image_url", "image_file"} {
		t.Run(typ, func(t *testing.T) {
			err := (&Service{DS: &uploadStub{}}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, map[string]any{
				"input": []any{map[string]any{"type": typ}},
			})
			status, _ := MapError(err)
			if err == nil || status != http.StatusBadRequest {
				t.Fatalf("declared image without a source must fail instead of losing the image: %v", err)
			}
		})
	}
}

func TestMalformedFileBlockIsRejectedInsteadOfSilentlyDropped(t *testing.T) {
	for _, block := range []map[string]any{
		{"type": "input_file"},
		{"type": "inline_file"},
		{"type": "file", "file": map[string]any{"filename": "missing.pdf"}},
	} {
		err := (&Service{DS: &uploadStub{}}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, map[string]any{
			"input": []any{block},
		})
		status, _ := MapError(err)
		if err == nil || status != http.StatusBadRequest {
			t.Fatalf("declared file without data, URL or ID must fail instead of losing the file: block=%v err=%v", block, err)
		}
	}
}

func TestImageFileReferenceDoesNotDisappear(t *testing.T) {
	uploader := &uploadStub{}
	a := &auth.RequestAuth{UseConfigToken: true, AccountID: "file-account"}
	req := map[string]any{"input": []any{map[string]any{"type": "image_file", "image_file": map[string]any{"file_id": "photo-existing"}}}}
	if err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), a, req); err != nil {
		t.Fatal(err)
	}
	refs, _ := req["ref_file_ids"].([]any)
	if len(uploader.requests) != 0 || len(refs) != 1 || refs[0] != "photo-existing" || a.TargetAccount != "file-account" {
		t.Fatalf("image_file reference was lost or uploaded again: uploads=%d refs=%v target=%s", len(uploader.requests), refs, a.TargetAccount)
	}
}

func TestImageFileRejectsUnrecognizedNestedID(t *testing.T) {
	err := (&Service{DS: &uploadStub{}}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, map[string]any{
		"input": []any{map[string]any{"type": "image_file", "image_file": map[string]any{"id": "not-a-file-id-field"}}},
	})
	if err == nil {
		t.Fatal("an unrecognized image_file id must not silently lose the reference")
	}
}

func TestAttachmentLimitCountsExistingAndNewReferences(t *testing.T) {
	for _, tc := range []struct {
		name        string
		existing    int
		inline      bool
		wantUploads int
		wantError   bool
	}{
		{name: "existing references over limit", existing: 51, inline: true, wantError: true},
		{name: "existing plus new image over limit", existing: 50, inline: true, wantUploads: 1, wantError: true},
		{name: "exact limit", existing: 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs := make([]any, tc.existing)
			for i := range refs {
				refs[i] = fmt.Sprintf("existing-%d", i)
			}
			req := map[string]any{"ref_file_ids": refs}
			if tc.inline {
				req["input"] = []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="}}
			}
			uploader := &uploadStub{}
			err := (&Service{DS: uploader}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected limit result: %v", err)
			}
			if err != nil {
				if status, _ := MapError(err); status != http.StatusBadRequest {
					t.Fatalf("limit error must be a bad request: %v", err)
				}
			}
			if len(uploader.requests) != tc.wantUploads {
				t.Fatalf("unexpected uploads before limit error: got=%d want=%d", len(uploader.requests), tc.wantUploads)
			}
		})
	}
}
