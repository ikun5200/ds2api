package inputfiles

import (
	"fmt"
	"net/http"
	"strings"
)

func validateAttachmentRefCount(ids []string) error {
	if len(ids) > maxInlineFilesPerRequest {
		return &inlineFileUploadError{status: http.StatusBadRequest, message: fmt.Sprintf("exceeded maximum of %d referenced files per request", maxInlineFilesPerRequest)}
	}
	return nil
}

func hasAttachmentReference(block map[string]any) bool {
	if strings.TrimSpace(asString(block["file_id"])) != "" {
		return true
	}
	if strings.Contains(strings.ToLower(asString(block["type"])), "file") && strings.TrimSpace(asString(block["id"])) != "" {
		return true
	}
	for _, key := range []string{"file", "image_file"} {
		if nested, ok := block[key].(map[string]any); ok {
			if strings.TrimSpace(asString(nested["file_id"])) != "" || (key == "file" && strings.TrimSpace(asString(nested["id"])) != "") {
				return true
			}
		}
	}
	return false
}

func attachmentContainerKeys(block map[string]any) []string {
	keys := []string{"messages", "input", "attachments", "content", "files", "items", "data", "source", "file", "image_file", "image_url"}
	switch strings.ToLower(strings.TrimSpace(asString(block["type"]))) {
	case "function_call_output", "tool_result":
		keys = append(keys, "output")
	}
	return keys
}
