package inputfiles

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
)

type uploadedFileFetcher interface {
	FetchUploadedFile(context.Context, *auth.RequestAuth, string) (*dsclient.UploadFileResult, error)
}

// ResolveFileReferences routes known references to their owning account and
// verifies access before a completion or retrieval can proceed. Unknown owners
// after a restart are checked with the current credential; they are never sent
// to a completion on a best-effort basis.
func ResolveFileReferences(ctx context.Context, a *auth.RequestAuth, backend any, ids []string) (map[string]*dsclient.UploadFileResult, error) {
	results := make(map[string]*dsclient.UploadFileResult, len(ids))
	if len(ids) == 0 {
		return results, nil
	}
	owner := ""
	for _, id := range ids {
		if known, ok := a.FileOwner(id); ok {
			if owner != "" && owner != known {
				return nil, &inlineFileUploadError{status: http.StatusConflict, message: "File references belong to different DeepSeek accounts; upload them to the same account."}
			}
			owner = known
		}
	}
	if a != nil && a.UseConfigToken {
		if owner == "" {
			owner = a.AccountID
		}
		if err := a.SelectFileAccount(ctx, owner); err != nil {
			if errors.Is(err, auth.ErrFileAccountConflict) {
				return nil, &inlineFileUploadError{status: http.StatusConflict, message: "File references conflict with the selected DeepSeek account; use their owning account or upload them again.", err: err}
			}
			return nil, &inlineFileUploadError{status: http.StatusServiceUnavailable, message: "The DeepSeek account that owns these files is unavailable.", err: err}
		}
	}
	fetcher, ok := backend.(uploadedFileFetcher)
	if !ok {
		return nil, &inlineFileUploadError{status: http.StatusNotImplemented, message: "File reference verification is not available."}
	}
	for _, id := range ids {
		if _, seen := results[id]; seen {
			continue
		}
		file, err := fetcher.FetchUploadedFile(ctx, a, id)
		if errors.Is(err, dsclient.ErrUploadFileNotFound) || (err == nil && (file == nil || file.ID != id)) {
			return nil, &inlineFileUploadError{status: http.StatusNotFound, message: "File not found or not accessible to the current DeepSeek account. Use X-Ds2-Target-Account with the upload's account_id, or upload the file again.", err: err}
		}
		if err != nil {
			return nil, &inlineFileUploadError{status: http.StatusBadGateway, message: "Failed to verify file access with DeepSeek; retry before generating a response.", err: err}
		}
		a.RememberFileOwner(id)
		results[id] = file
	}
	return results, nil
}

func validateFileReferencesReady(files map[string]*dsclient.UploadFileResult) error {
	for _, file := range files {
		if dsclient.IsUploadedFileReady(file.Status) {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(file.Status)) {
		case "FAILED", "CANCELLED", "CONTENT_FILTER", "CONTENT_TOO_LONG", "CONTENT_EMPTY":
			return &inlineFileUploadError{status: http.StatusBadRequest, message: "The referenced file failed processing; upload a valid file again."}
		default:
			return &inlineFileUploadError{status: http.StatusConflict, message: "The referenced file is still processing; wait for upload processing to complete before generating a response."}
		}
	}
	return nil
}
