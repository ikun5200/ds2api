package sse

import (
	"context"
	"strings"
	"testing"
)

const flashSearchSSE = `event: ready
data: {"request_message_id":1,"response_message_id":2,"model_type":"default"}

event: update_file
data: {"id":"file-test","file_name":"notes.txt","status":"SUCCESS","v":"file metadata"}

data: {"v":{"response":{"fragments":[{"id":2,"type":"THINK","content":""}],"search_triggered":true}}}
data: {"p":"response/fragments/-1/content","o":"APPEND","v":"正在"}
data: {"v":"检索。"}
data: {"p":"response/fragments","o":"APPEND","v":[{"id":3,"type":"TOOL_SEARCH","content":"search metadata","queries":[],"results":[]}]}
data: {"p":"response/fragments/-1","o":"BATCH","v":[{"p":"queries","o":"APPEND","v":["first query"]},{"v":["second query"]},{"p":"results","v":[{"url":"https://example.com/source","cite_index":1,"content":"search snippet"}]}]}
data: {"p":"response/fragments/-1/queries/-1","o":"APPEND","v":"hidden query"}
data: {"v":" continuation"}
data: {"p":"response/fragments/-1/results/-1/content","v":"hidden result"}
data: {"p":"response/fragments/-1/content","v":"hidden tool delta"}
data: {"v":" continuation"}
data: {"p":"response/fragments/-1/status","o":"SET","v":"FINISHED"}
data: {"p":"response/fragments","o":"APPEND","v":[{"id":4,"type":"RESPONSE","content":""}]}
data: {"p":"response/fragments/-1/content","v":"答案"}
data: {"v":"。"}
data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":"123"},{"v":"456"},{"p":"quasi_status","v":"FINISHED"}]}
data: {"p":"response/status","o":"SET","v":"FINISHED"}

event: finish
data: {}

event: close
data: {"click_behavior":"none"}
`

func TestFlashSearchSSEStreamAndNonstream(t *testing.T) {
	for _, searchType := range []string{"TOOL_SEARCH", "SEARCH"} {
		t.Run(searchType, func(t *testing.T) {
			body := strings.ReplaceAll(flashSearchSSE, `"type":"TOOL_SEARCH"`, `"type":"`+searchType+`"`)
			checkSearchSSEStreamAndNonstream(t, body)
		})
	}
}

func checkSearchSSEStreamAndNonstream(t *testing.T, body string) {
	t.Helper()
	for _, thinkingEnabled := range []bool{true, false} {
		collected := CollectStream(makeHTTPResponse(body), thinkingEnabled, false)
		if collected.Text != "答案。" || collected.ToolDetectionThinking != "正在检索。" || collected.ResponseMessageID != 2 {
			t.Fatalf("unexpected collected content: %#v", collected)
		}
		if collected.CitationLinks[1] != "https://example.com/source" {
			t.Fatalf("search citation lost: %#v", collected.CitationLinks)
		}
		wantThinking := ""
		if thinkingEnabled {
			wantThinking = "正在检索。"
		}
		if collected.Thinking != wantThinking {
			t.Fatalf("thinking=%q, want %q", collected.Thinking, wantThinking)
		}
		results, done := StartParsedLinePump(context.Background(), strings.NewReader(body), thinkingEnabled, "text")
		var text, thinking string
		var messageID int
		for result := range results {
			if result.ResponseMessageID > 0 {
				messageID = result.ResponseMessageID
			}
			for _, part := range result.Parts {
				if part.Type == "thinking" {
					thinking += part.Text
				} else {
					text += part.Text
				}
			}
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if text != collected.Text || thinking != wantThinking || messageID != 2 {
			t.Fatalf("stream mismatch: text=%q thinking=%q messageID=%d", text, thinking, messageID)
		}
	}
}

func TestCompressedBatchPreservesContentBeforeFinished(t *testing.T) {
	body := `data: {"p":"response","o":"BATCH","v":[{"p":"content","o":"APPEND","v":"final"},{"v":" answer"},{"p":"status","o":"SET","v":"FINISHED"}]}` + "\n"
	result := CollectStream(makeHTTPResponse(body), false, false)
	if result.Text != "final answer" {
		t.Fatalf("content in final batch lost: %#v", result)
	}
}

func TestCloseDoesNotEndAutomaticContinuation(t *testing.T) {
	body := "data: {\"p\":\"response/status\",\"v\":\"INCOMPLETE\"}\n" +
		"event: close\ndata: {}\n" +
		"event: ready\ndata: {\"response_message_id\":3}\n" +
		"data: {\"v\":\"continued answer\"}\n" +
		"data: {\"p\":\"response/status\",\"v\":\"FINISHED\"}\n"
	result := CollectStream(makeHTTPResponse(body), false, false)
	if result.Text != "continued answer" || result.ResponseMessageID != 3 {
		t.Fatalf("continuation lost after close: %#v", result)
	}
}
