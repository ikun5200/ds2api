package completionruntime

import (
	"context"
	"fmt"
	"testing"

	"ds2api/internal/auth"
	"ds2api/internal/httpapi/openai/history"
	"ds2api/internal/promptcompat"
)

func TestAutomaticContextFilesRespectAttachmentLimit(t *testing.T) {
	for _, tc := range []struct {
		refs    int
		tools   bool
		uploads int
	}{
		{50, false, 0}, {50, true, 0}, {49, true, 0}, {49, false, 1}, {48, true, 2},
	} {
		t.Run(fmt.Sprintf("refs=%d/tools=%v", tc.refs, tc.tools), func(t *testing.T) {
			ds := &fakeDeepSeekCaller{}
			std := promptcompat.StandardRequest{ResolvedModel: "deepseek-flash", FinalPrompt: "Full original conversation", Messages: []any{map[string]any{"role": "user", "content": "Analyze these images"}}}
			for i := 0; i < tc.refs; i++ {
				std.RefFileIDs = append(std.RefFileIDs, fmt.Sprintf("image-%d", i))
			}
			if tc.tools {
				std.ToolsRaw = []any{map[string]any{"type": "function", "function": map[string]any{"name": "weather", "parameters": map[string]any{"type": "object"}}}}
			}
			got, err := (history.Service{Store: currentInputRuntimeConfig{}, DS: ds}).ApplyCurrentInputFile(context.Background(), &auth.RequestAuth{}, std)
			if err != nil {
				t.Fatal(err)
			}
			if len(ds.uploads) != tc.uploads {
				t.Fatalf("uploads=%d want=%d", len(ds.uploads), tc.uploads)
			}
			if len(got.RefFileIDs) > promptcompat.MaxRefFileIDs {
				t.Fatalf("too many references: %d", len(got.RefFileIDs))
			}
			if tc.uploads == 0 && (got.FinalPrompt != std.FinalPrompt || got.CurrentInputFileApplied) {
				t.Fatal("fallback must keep complete original prompt")
			}
			for i := 0; i < tc.refs; i++ {
				id := fmt.Sprintf("image-%d", i)
				found := false
				for _, ref := range got.RefFileIDs {
					if ref == id {
						found = true
					}
				}
				if !found {
					t.Fatalf("client image was dropped: %s", id)
				}
			}
		})
	}
}
