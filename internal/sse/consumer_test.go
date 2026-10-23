package sse

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCollectStreamCapturesAccumulatedTokenUsage(t *testing.T) {
	body := strings.Join([]string{
		`data: {"p":"response/content","v":"hello"}`,
		``,
		`data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":321},{"p":"quasi_status","v":"FINISHED"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	got := CollectStream(resp, false, true)
	if got.Text != "hello" {
		t.Fatalf("unexpected text: %q", got.Text)
	}
	if got.AccumulatedTokenUsage != 321 {
		t.Fatalf("expected accumulated token usage 321, got %d", got.AccumulatedTokenUsage)
	}
}

func TestCollectStreamDedupesContinueSnapshotReplay(t *testing.T) {
	prefix := "我们被问到：这是一个很长的续答快照前缀，用来验证去重逻辑不会误伤正常 token。"
	body := strings.Join([]string{
		`data: {"v":{"response":{"fragments":[{"id":2,"type":"THINK","content":"` + prefix + `","references":[],"stage_id":1}]}}}`,
		``,
		`data: {"p":"response/status","v":"INCOMPLETE"}`,
		``,
		`data: {"v":{"response":{"fragments":[{"id":2,"type":"THINK","content":"` + prefix + `继续","references":[],"stage_id":1}]}}}`,
		``,
		`data: {"v":"分析"}`,
		``,
		`data: {"p":"response/status","v":"FINISHED"}`,
		``,
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	got := CollectStream(resp, true, true)
	if got.Thinking != prefix+"继续分析" {
		t.Fatalf("unexpected thinking after dedupe: %q", got.Thinking)
	}
}
