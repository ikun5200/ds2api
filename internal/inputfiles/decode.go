package inputfiles

import (
	"encoding/base64"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

func asString(v any) string { s, _ := v.(string); return s }

func decodeInlineFileBlock(block map[string]any) (inlineDecodedFile, bool, error) {
	if block == nil {
		return inlineDecodedFile{}, false, nil
	}
	if hasAttachmentReference(block) {
		return inlineDecodedFile{}, false, nil
	}
	if nested, ok := block["file"].(map[string]any); ok {
		decoded, matched, err := decodeInlineFileBlock(nested)
		if err != nil || !matched {
			return decoded, matched, err
		}
		if decoded.Filename == "" {
			decoded.Filename = pickInlineFilename(block, decoded.ContentType, defaultInlinePrefix(decoded.ReplacementType))
		}
		return decoded, true, nil
	}
	blockType := strings.ToLower(strings.TrimSpace(asString(block["type"])))
	_, hasFileData := block["file_data"]
	_, hasImageURL := block["image_url"]
	switch blockType {
	case "input_file", "inline_file", "file", "input_image", "image_url":
	default:
		// Structured tool results may legitimately contain name, data, or url.
		// Decode only an explicit attachment shape, not arbitrary business JSON.
		if !hasFileData && !hasImageURL {
			return inlineDecodedFile{}, false, nil
		}
	}
	if raw, matched := extractInlineImageDataURL(block); matched {
		data, contentType, err := decodeInlinePayload(raw, contentTypeFromMap(block))
		if err != nil {
			return inlineDecodedFile{}, true, fmt.Errorf("invalid image input")
		}
		return inlineDecodedFile{
			Data:            data,
			ContentType:     contentType,
			Filename:        pickInlineFilename(block, contentType, "image"),
			ReplacementType: "input_image",
		}, true, nil
	}
	if raw, matched := extractInlineFilePayload(block, blockType); matched {
		data, contentType, err := decodeInlinePayload(raw, contentTypeFromMap(block))
		if err != nil {
			return inlineDecodedFile{}, true, fmt.Errorf("invalid file input")
		}
		return inlineDecodedFile{
			Data:            data,
			ContentType:     contentType,
			Filename:        pickInlineFilename(block, contentType, defaultInlinePrefix(blockType)),
			ReplacementType: "input_file",
		}, true, nil
	}
	return inlineDecodedFile{}, false, nil
}

func extractInlineImageDataURL(block map[string]any) (string, bool) {
	imageURL := block["image_url"]
	switch x := imageURL.(type) {
	case string:
		if isDataURL(x) {
			return strings.TrimSpace(x), true
		}
	case map[string]any:
		if raw := strings.TrimSpace(asString(x["url"])); isDataURL(raw) {
			return raw, true
		}
	}
	if raw := strings.TrimSpace(asString(block["url"])); isDataURL(raw) {
		return raw, true
	}
	return "", false
}

func extractInlineFilePayload(block map[string]any, blockType string) (string, bool) {
	if value, exists := block["file_data"]; exists {
		return strings.TrimSpace(asString(value)), true
	}
	for _, value := range []any{block["file_data"], block["base64"], block["data"]} {
		if raw := strings.TrimSpace(asString(value)); raw != "" {
			if strings.Contains(blockType, "file") || block["file_data"] != nil || block["filename"] != nil || block["file_name"] != nil || block["name"] != nil {
				return raw, true
			}
		}
	}
	return "", false
}

func decodeInlinePayload(raw string, explicitContentType string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, "", fmt.Errorf("empty payload")
	}
	if isDataURL(raw) {
		return decodeDataURL(raw, explicitContentType)
	}
	decoded, err := decodeBase64Flexible(raw)
	if err != nil {
		return nil, "", err
	}
	contentType := strings.TrimSpace(explicitContentType)
	if contentType == "" && len(decoded) > 0 {
		contentType = http.DetectContentType(decoded)
	}
	return decoded, contentType, nil
}

func decodeDataURL(raw string, explicitContentType string) ([]byte, string, error) {
	raw = strings.TrimSpace(raw)
	if !isDataURL(raw) {
		return nil, "", fmt.Errorf("unsupported data url")
	}
	header, payload, ok := strings.Cut(raw, ",")
	if !ok {
		return nil, "", fmt.Errorf("invalid data url")
	}
	meta := strings.TrimSpace(header[len("data:"):])
	contentType := strings.TrimSpace(explicitContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
		if meta != "" {
			parts := strings.Split(meta, ";")
			if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
				contentType = strings.TrimSpace(parts[0])
			}
		}
	}
	if strings.Contains(strings.ToLower(meta), ";base64") {
		decoded, err := decodeBase64Flexible(payload)
		if err != nil {
			return nil, "", err
		}
		return decoded, contentType, nil
	}
	decoded, err := url.PathUnescape(payload)
	if err != nil {
		return nil, "", err
	}
	return []byte(decoded), contentType, nil
}

func decodeBase64Flexible(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		decoded, err := enc.DecodeString(raw)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, fmt.Errorf("invalid base64 payload")
}

func contentTypeFromMap(block map[string]any) string {
	for _, value := range []any{block["mime_type"], block["mimeType"], block["content_type"], block["contentType"], block["media_type"], block["mediaType"]} {
		if contentType := strings.TrimSpace(asString(value)); contentType != "" {
			return contentType
		}
	}
	if imageURL, ok := block["image_url"].(map[string]any); ok {
		for _, value := range []any{imageURL["mime_type"], imageURL["mimeType"], imageURL["content_type"], imageURL["contentType"]} {
			if contentType := strings.TrimSpace(asString(value)); contentType != "" {
				return contentType
			}
		}
	}
	return ""
}

func pickInlineFilename(block map[string]any, contentType string, prefix string) string {
	for _, value := range []any{block["filename"], block["file_name"], block["name"]} {
		if name := strings.TrimSpace(asString(value)); name != "" {
			return filepath.Base(name)
		}
	}
	if prefix == "" {
		prefix = "upload"
	}
	ext := ".bin"
	if parsedType := strings.TrimSpace(contentType); parsedType != "" {
		if comma := strings.Index(parsedType, ";"); comma >= 0 {
			parsedType = strings.TrimSpace(parsedType[:comma])
		}
		if exts, err := mime.ExtensionsByType(parsedType); err == nil && len(exts) > 0 && strings.TrimSpace(exts[0]) != "" {
			ext = exts[0]
		}
		if strings.EqualFold(parsedType, "image/jpeg") {
			ext = ".jpg"
		}
	}
	return prefix + ext
}

func defaultInlinePrefix(blockType string) string {
	blockType = strings.ToLower(strings.TrimSpace(blockType))
	if strings.Contains(blockType, "image") {
		return "image"
	}
	return "upload"
}

func isDataURL(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "data:")
}
