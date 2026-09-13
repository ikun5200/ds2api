package inputfiles

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"time"

	"ds2api/internal/config"
)

const maxAttachmentBytes = 100 << 20

// Resolve and dial a public address together so DNS changes cannot redirect a
// validated attachment URL to an internal service. Never use proxy environment
// variables for client-supplied attachment URLs.
var attachmentHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		DialContext:           dialPublicAttachment,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many attachment redirects")
		}
		return validateAttachmentURL(req.URL)
	},
}

func (s *inlineUploadState) decodeBlock(block map[string]any) (inlineDecodedFile, bool, error) {
	decoded, matched, err := decodeInlineFileBlock(block)
	if matched || err != nil || hasAttachmentReference(block) {
		return decoded, matched, err
	}
	rawURL, replacementType := attachmentRemoteURL(block)
	if rawURL == "" {
		if _, exists := block["image_url"]; exists {
			return inlineDecodedFile{}, true, fmt.Errorf("image URL is required")
		}
		if _, exists := block["file_url"]; exists {
			return inlineDecodedFile{}, true, fmt.Errorf("file URL is required")
		}
		switch strings.ToLower(strings.TrimSpace(asString(block["type"]))) {
		case "input_image", "image_url", "image_file":
			return inlineDecodedFile{}, true, fmt.Errorf("image input requires an image URL or file ID")
		case "input_file", "inline_file", "file":
			return inlineDecodedFile{}, true, fmt.Errorf("file input requires file data, a file URL or file ID")
		}
		return inlineDecodedFile{}, false, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || validateAttachmentURL(u) != nil {
		return inlineDecodedFile{}, true, fmt.Errorf("file URL must use public HTTP or HTTPS")
	}
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return inlineDecodedFile{}, true, fmt.Errorf("invalid file URL")
	}
	resp, err := attachmentHTTPClient.Do(req)
	if err != nil {
		return inlineDecodedFile{}, true, fmt.Errorf("failed to download file URL")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			config.Logger.Warn("[input_files] failed to close attachment response", "error", err)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return inlineDecodedFile{}, true, fmt.Errorf("file URL returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxAttachmentBytes {
		return inlineDecodedFile{}, true, fmt.Errorf("file input exceeds 100 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes+1))
	if err != nil {
		return inlineDecodedFile{}, true, fmt.Errorf("failed to read file URL")
	}
	if len(data) > maxAttachmentBytes {
		return inlineDecodedFile{}, true, fmt.Errorf("file input exceeds 100 MiB")
	}
	contentType := contentTypeFromMap(block)
	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	filenameBlock := map[string]any{"filename": block["filename"]}
	if asString(filenameBlock["filename"]) == "" {
		if name := path.Base(u.Path); name != "." && name != "/" && name != "" {
			filenameBlock["filename"] = name
		}
	}
	return inlineDecodedFile{
		Data: data, ContentType: contentType,
		Filename:        pickInlineFilename(filenameBlock, contentType, defaultInlinePrefix(replacementType)),
		ReplacementType: replacementType,
	}, true, nil
}

func attachmentRemoteURL(block map[string]any) (string, string) {
	blockType := strings.ToLower(strings.TrimSpace(asString(block["type"])))
	if blockType == "image_url" || blockType == "input_image" {
		if nested, ok := block["image_url"].(map[string]any); ok {
			return strings.TrimSpace(asString(nested["url"])), "input_image"
		}
		return strings.TrimSpace(asString(block["image_url"])), "input_image"
	}
	if blockType == "file" || blockType == "input_file" {
		if nested, ok := block["file"].(map[string]any); ok {
			return strings.TrimSpace(asString(nested["file_url"])), "input_file"
		}
		return strings.TrimSpace(asString(block["file_url"])), "input_file"
	}
	return "", ""
}

func validateAttachmentURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("unsupported attachment URL")
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && !isPublicAttachmentIP(ip) {
		return fmt.Errorf("attachment URL must use a public address")
	}
	return nil
}

func isPublicAttachmentIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	addr = addr.Unmap()
	for _, reserved := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(reserved).Contains(addr) {
			return false
		}
	}
	return true
}

func dialPublicAttachment(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, addr := range addresses {
		if !isPublicAttachmentIP(addr.IP) {
			return nil, fmt.Errorf("attachment URL must use a public address")
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	for _, addr := range addresses {
		conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(addr.IP.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		err = dialErr
	}
	if err == nil {
		err = fmt.Errorf("attachment hostname has no public address")
	}
	return nil, err
}
