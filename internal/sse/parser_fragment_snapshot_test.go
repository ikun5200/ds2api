package sse

import "testing"

// DeepSeek may finish a thinking-only stream with a full fragments snapshot
// that has no "o" field and contains both the complete THINK fragment and the
// RESPONSE fragment. The THINK text was already streamed, so only the RESPONSE
// content should be recovered from the snapshot.
func TestParseSSEChunkForContentRecoversResponseFromFragmentSnapshot(t *testing.T) {
	chunk := map[string]any{
		"p": "response/fragments",
		"v": []any{
			map[string]any{"id": float64(2), "type": "THINK", "content": "reasoning ... Use 73."},
			map[string]any{"id": float64(3), "type": "RESPONSE", "content": "73"},
		},
	}

	parts, detectionThinking, finished, nextType := ParseSSEChunkForContentDetailed(chunk, true, "thinking")
	if finished {
		t.Fatal("fragment snapshot must not mark the stream finished")
	}
	if nextType != "text" {
		t.Fatalf("nextType=%q want text", nextType)
	}
	if len(detectionThinking) != 0 {
		t.Fatalf("snapshot THINK must not be duplicated: %#v", detectionThinking)
	}
	if len(parts) != 1 || parts[0].Type != "text" || parts[0].Text != "73" {
		t.Fatalf("expected only the RESPONSE fragment, got %#v", parts)
	}
}

func TestParseSSEChunkForContentSkipsDuplicateResponseSnapshot(t *testing.T) {
	chunk := map[string]any{
		"p": "response/fragments",
		"v": []any{
			map[string]any{"type": "RESPONSE", "content": "already streamed"},
		},
	}

	parts, _, _, _ := ParseSSEChunkForContentDetailed(chunk, false, "text")
	if len(parts) != 0 {
		t.Fatalf("expected duplicate snapshot to be ignored, got %#v", parts)
	}
}

func TestParseSSEChunkForContentStillHandlesAppendFragments(t *testing.T) {
	chunk := map[string]any{
		"p": "response/fragments",
		"o": "APPEND",
		"v": []any{
			map[string]any{"type": "THINK", "content": "thinking"},
			map[string]any{"type": "RESPONSE", "content": "answer"},
		},
	}

	parts, detectionThinking, _, nextType := ParseSSEChunkForContentDetailed(chunk, true, "thinking")
	if nextType != "text" {
		t.Fatalf("nextType=%q want text", nextType)
	}
	if len(detectionThinking) != 1 || detectionThinking[0].Text != "thinking" {
		t.Fatalf("expected thinking fragment, got %#v", detectionThinking)
	}
	if len(parts) != 2 || parts[0].Type != "thinking" || parts[1].Type != "text" || parts[1].Text != "answer" {
		t.Fatalf("unexpected parts: %#v", parts)
	}
}
