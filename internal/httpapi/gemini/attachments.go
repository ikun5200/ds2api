package gemini

import "strings"

func attachGeminiMessageFiles(messages []any, start int, parts []any) []any {
	attachments := collectGeminiAttachmentParts(parts)
	if len(attachments) == 0 || len(messages) <= start {
		return messages
	}
	message, _ := messages[len(messages)-1].(map[string]any)
	existing, _ := message["attachments"].([]any)
	message["attachments"] = append(existing, attachments...)
	return messages
}

func attachGeminiFunctionResponseFiles(message, response map[string]any) map[string]any {
	parts, _ := response["parts"].([]any)
	if attachments := collectGeminiAttachmentParts(parts); len(attachments) > 0 {
		message["attachments"] = attachments
	}
	return message
}

func collectGeminiAttachmentParts(parts []any) []any {
	attachments := make([]any, 0)
	for _, item := range parts {
		part, _ := item.(map[string]any)
		if block := normalizeGeminiAttachmentPart(part); block != nil {
			attachments = append(attachments, block)
		}
	}
	return attachments
}

func normalizeGeminiAttachmentPart(part map[string]any) map[string]any {
	for _, key := range []string{"inlineData", "inline_data", "fileData", "file_data"} {
		data, ok := part[key].(map[string]any)
		if !ok {
			continue
		}
		contentType := strings.TrimSpace(asString(data["mimeType"]))
		if contentType == "" {
			contentType = strings.TrimSpace(asString(data["mime_type"]))
		}
		out := map[string]any{"type": "input_file", "mime_type": contentType}
		isImage := strings.HasPrefix(strings.ToLower(contentType), "image/")
		if isImage {
			out["type"] = "input_image"
		}
		if fileID := strings.TrimSpace(asString(data["file_id"])); fileID != "" {
			out["file_id"] = fileID
			return out
		}
		if key == "inlineData" || key == "inline_data" {
			if isImage {
				out["image_url"] = "data:" + contentType + ";base64," + asString(data["data"])
			} else {
				out["file_data"] = asString(data["data"])
			}
		} else {
			uri := strings.TrimSpace(asString(data["fileUri"]))
			if uri == "" {
				uri = strings.TrimSpace(asString(data["file_uri"]))
			}
			if isImage {
				out["image_url"] = uri
			} else {
				out["file_url"] = uri
			}
		}
		return out
	}
	return nil
}
