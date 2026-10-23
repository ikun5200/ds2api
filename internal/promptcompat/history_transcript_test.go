package promptcompat

import (
	"strings"
	"testing"
)

func TestBuildOpenAIHistoryTranscriptInfersToolNameFromCallID(t *testing.T) {
	messages := []any{
		map[string]any{
			"role": "assistant",
			"tool_calls": []any{
				map[string]any{
					"id":   "call_1",
					"type": "function",
					"function": map[string]any{
						"name":      "get_weather",
						"arguments": `{"city":"北京"}`,
					},
				},
			},
		},
		map[string]any{
			"role":         "tool",
			"tool_call_id": "call_1",
			"content":      `{"temperature_c":18}`,
		},
	}

	got := BuildOpenAIHistoryTranscript(messages)
	if !strings.Contains(got, "[name=get_weather tool_call_id=call_1]") {
		t.Fatalf("expected inferred tool name in history header, got %q", got)
	}
	if !strings.Contains(got, `{"temperature_c":18}`) {
		t.Fatalf("expected tool result content preserved, got %q", got)
	}
}

func TestBuildOpenAIHistoryTranscriptPreservesLegacyFunctionCall(t *testing.T) {
	messages := []any{
		map[string]any{
			"role": "assistant",
			"function_call": map[string]any{
				"name":      "search",
				"arguments": `{"query":"docs"}`,
			},
		},
		map[string]any{
			"role":    "function",
			"name":    "search",
			"content": `{"hits":1}`,
		},
	}

	got := BuildOpenAIHistoryTranscript(messages)
	if !strings.Contains(got, `<invoke name="search">`) {
		t.Fatalf("expected legacy function_call rendered as tool XML, got %q", got)
	}
	if !strings.Contains(got, "[name=search]") {
		t.Fatalf("expected legacy function result name preserved, got %q", got)
	}
}
