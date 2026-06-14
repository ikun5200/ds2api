package promptcompat

import (
	"ds2api/internal/prompt"
)

func buildOpenAIFinalPrompt(messagesRaw []any, toolsRaw any, traceID string, thinkingEnabled bool) (string, []string) {
	return BuildOpenAIPrompt(messagesRaw, toolsRaw, traceID, DefaultToolChoicePolicy(), thinkingEnabled)
}

func BuildOpenAIPrompt(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool) (string, []string) {
	return BuildOpenAIPromptWithPrepareOptions(messagesRaw, toolsRaw, traceID, toolPolicy, thinkingEnabled, prompt.DefaultPrepareOptions())
}

func BuildOpenAIPromptWithPrepareOptions(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool, prepareOptions prompt.PrepareOptions) (string, []string) {
	return buildOpenAIPrompt(messagesRaw, toolsRaw, traceID, toolPolicy, thinkingEnabled, true, prepareOptions)
}

func BuildOpenAIPromptWithToolInstructionsOnly(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool) (string, []string) {
	return BuildOpenAIPromptWithToolInstructionsOnlyOptions(messagesRaw, toolsRaw, traceID, toolPolicy, thinkingEnabled, prompt.DefaultPrepareOptions())
}

func BuildOpenAIPromptWithToolInstructionsOnlyOptions(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool, prepareOptions prompt.PrepareOptions) (string, []string) {
	return buildOpenAIPrompt(messagesRaw, toolsRaw, traceID, toolPolicy, thinkingEnabled, false, prepareOptions)
}

func buildOpenAIPrompt(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool, includeToolDescriptions bool, prepareOptions prompt.PrepareOptions) (string, []string) {
	messages := NormalizeOpenAIMessagesForPrompt(messagesRaw, traceID)
	toolNames := []string{}
	if tools, ok := toolsRaw.([]any); ok && len(tools) > 0 {
		if includeToolDescriptions {
			messages, toolNames = injectToolPrompt(messages, tools, toolPolicy)
		} else {
			messages, toolNames = injectToolPromptInstructionsOnly(messages, tools, toolPolicy)
		}
	}
	return prompt.MessagesPrepareWithThinkingOptions(messages, thinkingEnabled, prepareOptions), toolNames
}

// BuildOpenAIPromptForAdapter exposes the OpenAI-compatible prompt building flow so
// other protocol adapters (for example Gemini) can reuse the same tool/history
// normalization logic and remain behavior-compatible with chat/completions.
func BuildOpenAIPromptForAdapter(messagesRaw []any, toolsRaw any, traceID string, thinkingEnabled bool) (string, []string) {
	return buildOpenAIFinalPrompt(messagesRaw, toolsRaw, traceID, thinkingEnabled)
}

func BuildOpenAIPromptForAdapterWithPrepareOptions(messagesRaw []any, toolsRaw any, traceID string, thinkingEnabled bool, prepareOptions prompt.PrepareOptions) (string, []string) {
	return BuildOpenAIPromptWithPrepareOptions(messagesRaw, toolsRaw, traceID, DefaultToolChoicePolicy(), thinkingEnabled, prepareOptions)
}
