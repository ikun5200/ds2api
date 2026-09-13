package promptcompat

import "testing"

func TestCollectRefFileIDsDoesNotTreatBinarySourceAsFileID(t *testing.T) {
	got := CollectOpenAIRefFileIDs(map[string]any{"messages": []any{map[string]any{
		"role": "user", "content": []any{
			map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "QUJDRA=="}},
			map[string]any{"type": "document", "source": map[string]any{"type": "file", "file_id": "file-existing"}},
		},
	}}})
	if len(got) != 1 || got[0] != "file-existing" {
		t.Fatalf("unexpected references: %v", got)
	}
}
