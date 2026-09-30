package promptcompat

import (
	"ds2api/internal/prompt"
)

func BuildOpenAIPromptWithPrepareOptions(messagesRaw []any, toolsRaw any, traceID string, toolPolicy ToolChoicePolicy, thinkingEnabled bool, prepareOptions prompt.PrepareOptions) (string, []string) {
	return buildOpenAIPrompt(messagesRaw, toolsRaw, traceID, toolPolicy, thinkingEnabled, true, prepareOptions)
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
