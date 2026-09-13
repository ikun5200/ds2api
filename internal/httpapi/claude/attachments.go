package claude

import (
	"encoding/base64"
	"strings"
)

// Keep attachment bytes in standard blocks alongside the prompt-visible
// metadata, so the shared uploader can process them after normalization.
func attachClaudeMessageFiles(messages []any, start int, content any) []any {
	attachments := collectClaudeAttachmentBlocks(content)
	if len(attachments) == 0 || len(messages) <= start {
		return messages
	}
	message, _ := messages[len(messages)-1].(map[string]any)
	message["attachments"] = attachments
	return messages
}

func collectClaudeAttachmentBlocks(content any) []any {
	items, _ := content.([]any)
	var out []any
	for _, item := range items {
		block, _ := item.(map[string]any)
		typ := strings.ToLower(strings.TrimSpace(safeStringValue(block["type"])))
		if typ == "tool_result" {
			out = append(out, collectClaudeAttachmentBlocks(block["content"])...)
			continue
		}
		if normalized := normalizeClaudeAttachmentBlock(block); normalized != nil {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeClaudeAttachmentBlock(block map[string]any) map[string]any {
	typ := strings.ToLower(strings.TrimSpace(safeStringValue(block["type"])))
	if typ == "input_image" || typ == "input_file" || typ == "file" || typ == "image_url" {
		return cloneMap(block)
	}
	if typ != "image" && typ != "document" {
		return nil
	}
	source, _ := block["source"].(map[string]any)
	contentType := safeStringValue(source["media_type"])
	if contentType == "" && typ == "image" {
		contentType = "image/png"
	}
	out := map[string]any{"type": "input_file", "mime_type": contentType}
	if typ == "image" {
		out["type"] = "input_image"
	}
	if filename := safeStringValue(block["title"]); filename != "" {
		out["filename"] = filename
	}
	if fileID := safeStringValue(source["file_id"]); fileID != "" {
		out["file_id"] = fileID
		return out
	}
	switch safeStringValue(source["type"]) {
	case "url":
		if typ == "image" {
			out["image_url"] = safeStringValue(source["url"])
		} else {
			out["file_url"] = safeStringValue(source["url"])
		}
	case "text":
		out["file_data"] = base64.StdEncoding.EncodeToString([]byte(safeStringValue(source["data"])))
		if contentType == "" {
			out["mime_type"] = "text/plain"
		}
	default:
		if typ == "image" {
			out["image_url"] = "data:" + contentType + ";base64," + safeStringValue(source["data"])
		} else {
			out["file_data"] = safeStringValue(source["data"])
		}
	}
	return out
}
