package inputfiles

import (
	"context"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type metadataUploadStub struct {
	uploadStub
	files map[string]*dsclient.UploadFileResult
}

func (s *metadataUploadStub) FetchUploadedFile(_ context.Context, _ *auth.RequestAuth, id string) (*dsclient.UploadFileResult, error) {
	return s.files[id], nil
}

func TestReusedReferencesIncludeVerifiedTokenUsage(t *testing.T) {
	stub := &metadataUploadStub{files: map[string]*dsclient.UploadFileResult{
		"image":    {ID: "image", Status: "SUCCESS", TokenUsage: 317, Bytes: 9000},
		"document": {ID: "document", Status: "SUCCESS", Bytes: 90},
	}}
	req := map[string]any{
		"ref_file_ids": []any{"image", "document", "image"},
		"messages": []any{map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "input_image", "file_id": "image"},
		}}},
	}
	if err := (&Service{DS: stub}).PreprocessInlineFileInputs(context.Background(), &auth.RequestAuth{}, req); err != nil {
		t.Fatal(err)
	}
	if got := req["_inline_file_tokens"]; got != 347 {
		t.Fatalf("reused attachments lost verified token usage: got=%v want=347", got)
	}
	if len(stub.requests) != 0 {
		t.Fatal("existing references must not upload again")
	}
}

func TestReferenceAndInlineUploadCountSharedFileIDOnce(t *testing.T) {
	stub := &metadataUploadStub{files: map[string]*dsclient.UploadFileResult{
		"file-1": {ID: "file-1", Status: "SUCCESS", TokenUsage: 117},
		"other":  {ID: "other", Status: "SUCCESS", TokenUsage: 23},
	}}
	service := &Service{DS: stub}
	std := promptcompat.StandardRequest{
		ResolvedModel: "deepseek-flash", RefFileIDs: []string{"file-1", "other"},
		Messages: []any{map[string]any{"role": "user", "attachments": []any{
			map[string]any{"type": "input_image", "image_url": "data:image/png;base64,QUJDRA=="},
		}}},
	}
	got, err := service.Apply(context.Background(), &auth.RequestAuth{}, std)
	if err != nil {
		t.Fatal(err)
	}
	if got.RefFileTokens != 140 || len(got.RefFileIDs) != 2 || len(stub.requests) != 1 {
		t.Fatalf("shared IDs must count once: tokens=%d refs=%v uploads=%d", got.RefFileTokens, got.RefFileIDs, len(stub.requests))
	}
	// A standard request may already contain uploaded references; revalidation
	// must replace their verified total instead of counting their tokens twice.
	again, err := service.Apply(context.Background(), &auth.RequestAuth{}, got)
	if err != nil {
		t.Fatal(err)
	}
	if again.RefFileTokens != 140 || len(stub.requests) != 1 {
		t.Fatalf("revalidating references changed their token total: tokens=%d uploads=%d", again.RefFileTokens, len(stub.requests))
	}
}
