package util

import (
	"ds2api/internal/prompt"
)

func MessagesPrepare(messages []map[string]any) string {
	return prompt.MessagesPrepare(messages)
}

// EstimateTokens provides a rough token count approximation.
// For ASCII text (English, code, etc.) we use ~4 chars per token.
// For non-ASCII text (Chinese, Japanese, Korean, etc.) we use ~1.3 chars per token,
// which better reflects typical BPE tokenizer behavior for CJK scripts.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	asciiChars := 0
	nonASCIIChars := 0
	for _, r := range text {
		if r < 128 {
			asciiChars++
		} else {
			nonASCIIChars++
		}
	}
	// ASCII: ~4 chars per token; non-ASCII (CJK): ~1.3 chars per token
	n := asciiChars/4 + (nonASCIIChars*10+7)/13
	if n < 1 {
		return 1
	}
	return n
}
