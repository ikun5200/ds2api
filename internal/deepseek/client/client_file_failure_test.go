package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"ds2api/internal/auth"
)

func TestUploadedFileTerminalFailureStopsPolling(t *testing.T) {
	oldSleep := fileReadySleep
	fileReadySleep = func(time.Duration) {}
	t.Cleanup(func() { fileReadySleep = oldSleep })
	for _, status := range []string{"FAILED", "CONTENT_FILTER", "CONTENT_TOO_LONG", "CANCELLED", "CONTENT_EMPTY"} {
		t.Run(status, func(t *testing.T) {
			calls := 0
			client := &Client{regular: doerFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				payload := `{"code":0,"data":{"biz_code":0,"biz_data":{"files":[{"id":"file-photo","status":"` + status + `"}]}}}`
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(payload)), Request: req}, nil
			})}
			err := client.waitForUploadedFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, &UploadFileResult{ID: "file-photo", Status: "PENDING"})
			if err == nil || !strings.Contains(err.Error(), status) {
				t.Fatalf("expected processing failure to retain terminal status %s, got %v", status, err)
			}
			if calls != 1 {
				t.Fatalf("terminal status %s should stop polling immediately, fetched %d times", status, calls)
			}
		})
	}
}

func TestInitialUploadedFileFailureDoesNotPoll(t *testing.T) {
	calls := 0
	client := &Client{regular: doerFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, nil
	})}
	err := client.waitForUploadedFile(context.Background(), &auth.RequestAuth{}, &UploadFileResult{ID: "photo-rejected", Status: "CONTENT_EMPTY"})
	if err == nil || !strings.Contains(err.Error(), "CONTENT_EMPTY") || calls != 0 {
		t.Fatalf("upload response already has a terminal error: err=%v polls=%d", err, calls)
	}
}
