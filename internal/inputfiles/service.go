package inputfiles

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type Uploader interface {
	UploadFile(context.Context, *auth.RequestAuth, dsclient.UploadFileRequest, int) (*dsclient.UploadFileResult, error)
}

// Service handles normalized attachment blocks for every protocol adapter.
type Service struct {
	Store config.ModelAliasReader
	DS    Uploader
}

func (h *Service) Apply(ctx context.Context, a *auth.RequestAuth, req promptcompat.StandardRequest) (promptcompat.StandardRequest, error) {
	model := req.ResolvedModel
	if model == "" {
		model = req.RequestedModel
	}
	input := map[string]any{"model": model, "messages": req.Messages, "ref_file_ids": req.RefFileIDs, "thinking_enabled": req.Thinking}
	if err := h.PreprocessInlineFileInputs(ctx, a, input); err != nil {
		return req, err
	}
	req.Messages, _ = input["messages"].([]any)
	req.RefFileIDs = promptcompat.CollectOpenAIRefFileIDs(input)
	if tokens, ok := input["_inline_file_tokens"].(int); ok {
		req.RefFileTokens = tokens
	}
	return req, nil
}

func MapError(err error) (int, string) {
	if e, ok := err.(*inlineFileUploadError); ok {
		return e.status, e.Error()
	}
	return http.StatusInternalServerError, "Failed to process file input."
}

const maxInlineFilesPerRequest = promptcompat.MaxRefFileIDs

type inlineFileUploadError struct {
	status  int
	message string
	err     error
}

func (e *inlineFileUploadError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.message) != "" {
		return e.message
	}
	if e.err != nil {
		return e.err.Error()
	}
	return "inline file processing failed"
}

type inlineUploadState struct {
	ctx              context.Context
	handler          *Service
	auth             *auth.RequestAuth
	modelType        string
	thinkingEnabled  bool
	uploadedByID     map[string]string
	uploadedTokens   map[string]int
	countedIDs       map[string]bool
	uploadCount      int
	inlineFileBytes  int
	inlineFileTokens int
}

type inlineDecodedFile struct {
	Data            []byte
	ContentType     string
	Filename        string
	ReplacementType string
}

func (h *Service) PreprocessInlineFileInputs(ctx context.Context, a *auth.RequestAuth, req map[string]any) error {
	if h == nil || h.DS == nil || len(req) == 0 {
		return nil
	}
	var verifiedFiles map[string]*dsclient.UploadFileResult
	if refIDs := promptcompat.CollectOpenAIRefFileIDs(req); len(refIDs) > 0 {
		if err := validateAttachmentRefCount(refIDs); err != nil {
			return err
		}
		files, err := ResolveFileReferences(ctx, a, h.DS, refIDs)
		if err != nil {
			return err
		}
		if err := validateFileReferencesReady(files); err != nil {
			return err
		}
		verifiedFiles = files
	}
	modelType := "default"
	modeModel := config.DefaultDeepSeekModel
	if requestedModel, ok := req["model"].(string); ok {
		if resolvedModel, ok := config.ResolveModel(h.Store, requestedModel); ok {
			modeModel = resolvedModel
			if resolvedType, ok := config.GetModelType(resolvedModel); ok {
				modelType = resolvedType
			}
		}
	}
	thinkingEnabled, _ := promptcompat.ResolveRequestModes(req, modeModel)
	state := &inlineUploadState{
		ctx:             ctx,
		handler:         h,
		auth:            a,
		modelType:       modelType,
		thinkingEnabled: thinkingEnabled,
		uploadedByID:    map[string]string{},
		uploadedTokens:  map[string]int{},
		countedIDs:      map[string]bool{},
	}
	for id, file := range verifiedFiles {
		state.inlineFileTokens += attachmentTokenUsage(file)
		state.countedIDs[id] = true
	}
	for _, key := range []string{"messages", "input", "attachments"} {
		if raw, ok := req[key]; ok {
			updated, err := state.walk(raw)
			if err != nil {
				return err
			}
			req[key] = updated
		}
	}
	if refIDs := promptcompat.CollectOpenAIRefFileIDs(req); len(refIDs) > 0 {
		if err := validateAttachmentRefCount(refIDs); err != nil {
			return err
		}
		pinAttachmentAccount(a)
		refs := make([]any, len(refIDs))
		for i, fileID := range refIDs {
			refs[i] = fileID
		}
		req["ref_file_ids"] = refs
	}
	if state.inlineFileBytes > 0 {
		req["_inline_file_bytes"] = state.inlineFileBytes
	}
	if len(state.countedIDs) > 0 {
		req["_inline_file_tokens"] = state.inlineFileTokens
	}
	return nil
}

func (s *inlineUploadState) walk(raw any) (any, error) {
	switch x := raw.(type) {
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			updated, err := s.walk(item)
			if err != nil {
				return nil, err
			}
			out[i] = updated
		}
		return out, nil
	case map[string]any:
		if replacement, replaced, err := s.tryUploadBlock(x); replaced || err != nil {
			return replacement, err
		}
		for _, key := range attachmentContainerKeys(x) {
			if hasAttachmentReference(x) && (key == "file" || key == "image_file" || key == "image_url") {
				continue
			}
			if nested, ok := x[key]; ok {
				updated, err := s.walk(nested)
				if err != nil {
					return nil, err
				}
				x[key] = updated
			}
		}
		return x, nil
	default:
		return raw, nil
	}
}

func (s *inlineUploadState) tryUploadBlock(block map[string]any) (map[string]any, bool, error) {
	decoded, ok, err := s.decodeBlock(block)
	if err != nil {
		return nil, true, &inlineFileUploadError{status: http.StatusBadRequest, message: err.Error(), err: err}
	}
	if !ok {
		return nil, false, nil
	}
	if len(decoded.Data) == 0 || len(decoded.Data) > maxAttachmentBytes {
		return nil, true, &inlineFileUploadError{status: http.StatusBadRequest, message: "file input must contain between 1 byte and 100 MiB"}
	}
	if s.uploadCount >= maxInlineFilesPerRequest {
		err := fmt.Errorf("exceeded maximum of %d inline files per request", maxInlineFilesPerRequest)
		return nil, true, &inlineFileUploadError{status: http.StatusBadRequest, message: err.Error(), err: err}
	}
	pinAttachmentAccount(s.auth)
	accountID := ""
	if s.auth != nil {
		accountID = s.auth.AccountID
	}
	fileID, err := s.uploadInlineFile(decoded)
	if s.auth != nil && s.auth.AccountID != accountID {
		return nil, true, &inlineFileUploadError{status: http.StatusInternalServerError, message: "File inputs must remain on the same DeepSeek account; retry the request."}
	}
	if err != nil {
		return nil, true, &inlineFileUploadError{status: http.StatusInternalServerError, message: "Failed to upload inline file.", err: err}
	}
	s.uploadCount++
	s.inlineFileBytes += len(decoded.Data)
	if !s.countedIDs[fileID] {
		s.inlineFileTokens += s.uploadedTokens[fileID]
		s.countedIDs[fileID] = true
	}
	replacement := map[string]any{
		"type":    decoded.ReplacementType,
		"file_id": fileID,
	}
	if decoded.Filename != "" {
		replacement["filename"] = decoded.Filename
	}
	if decoded.ContentType != "" {
		replacement["mime_type"] = decoded.ContentType
	}
	return replacement, true, nil
}

func (s *inlineUploadState) uploadInlineFile(file inlineDecodedFile) (string, error) {
	sum := sha256.Sum256(append([]byte(file.ContentType+"\x00"+file.Filename+"\x00"), file.Data...))
	cacheKey := fmt.Sprintf("%x", sum[:])
	if fileID, ok := s.uploadedByID[cacheKey]; ok && strings.TrimSpace(fileID) != "" {
		return fileID, nil
	}
	contentType := strings.TrimSpace(file.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(file.Data)
	}
	result, err := s.handler.DS.UploadFile(s.ctx, s.auth, dsclient.UploadFileRequest{
		Filename:        file.Filename,
		ContentType:     contentType,
		ModelType:       s.modelType,
		Data:            file.Data,
		ThinkingEnabled: &s.thinkingEnabled,
	}, 3)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("upload succeeded without file metadata")
	}
	fileID := strings.TrimSpace(result.ID)
	if fileID == "" {
		return "", fmt.Errorf("upload succeeded without file id")
	}
	s.auth.RememberFileOwner(fileID)
	s.uploadedByID[cacheKey] = fileID
	if result.Bytes <= 0 {
		result.Bytes = int64(len(file.Data))
	}
	s.uploadedTokens[fileID] = attachmentTokenUsage(result)
	return fileID, nil
}

func attachmentTokenUsage(file *dsclient.UploadFileResult) int {
	if file.TokenUsage > 0 {
		return file.TokenUsage
	}
	if file.Bytes > 0 {
		return int(file.Bytes / 3)
	}
	return 0
}

func pinAttachmentAccount(a *auth.RequestAuth) {
	if a != nil && a.UseConfigToken && a.TargetAccount == "" && a.AccountID != "" {
		a.TargetAccount = a.AccountID
	}
}
