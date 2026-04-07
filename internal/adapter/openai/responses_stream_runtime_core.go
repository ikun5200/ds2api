package openai

import (
	"ds2api/internal/toolcall"
	"net/http"
	"strings"

	"ds2api/internal/config"
	openaifmt "ds2api/internal/format/openai"
	"ds2api/internal/sse"
	streamengine "ds2api/internal/stream"
	"ds2api/internal/util"
)

type responsesStreamRuntime struct {
	w        http.ResponseWriter
	rc       *http.ResponseController
	canFlush bool

	responseID  string
	model       string
	finalPrompt string
	toolNames   []string
	traceID     string
	toolChoice  util.ToolChoicePolicy

	thinkingEnabled       bool
	searchEnabled         bool
	stripReferenceMarkers bool

	bufferToolContent    bool
	emitEarlyToolDeltas  bool
	toolCallsEmitted     bool
	toolCallsDoneEmitted bool

	sieve             toolStreamSieveState
	thinking          strings.Builder
	text              strings.Builder
	visibleText       strings.Builder
	streamToolCallIDs map[int]string
	functionItemIDs   map[int]string
	functionOutputIDs map[int]int
	functionArgs      map[int]string
	functionDone      map[int]bool
	functionAdded     map[int]bool
	functionNames     map[int]string
	messageItemID     string
	messageOutputID   int
	nextOutputID      int
	messageAdded      bool
	messagePartAdded  bool
	sequence          int
	failed            bool
	promptTokens      int
	outputTokens      int

	persistResponse func(obj map[string]any)
}

func newResponsesStreamRuntime(
	w http.ResponseWriter,
	rc *http.ResponseController,
	canFlush bool,
	responseID string,
	model string,
	finalPrompt string,
	thinkingEnabled bool,
	searchEnabled bool,
	stripReferenceMarkers bool,
	toolNames []string,
	bufferToolContent bool,
	emitEarlyToolDeltas bool,
	toolChoice util.ToolChoicePolicy,
	traceID string,
	persistResponse func(obj map[string]any),
) *responsesStreamRuntime {
	return &responsesStreamRuntime{
		w:                     w,
		rc:                    rc,
		canFlush:              canFlush,
		responseID:            responseID,
		model:                 model,
		finalPrompt:           finalPrompt,
		thinkingEnabled:       thinkingEnabled,
		searchEnabled:         searchEnabled,
		stripReferenceMarkers: stripReferenceMarkers,
		toolNames:             toolNames,
		bufferToolContent:     bufferToolContent,
		emitEarlyToolDeltas:   emitEarlyToolDeltas,
		streamToolCallIDs:     map[int]string{},
		functionItemIDs:       map[int]string{},
		functionOutputIDs:     map[int]int{},
		functionArgs:          map[int]string{},
		functionDone:          map[int]bool{},
		functionAdded:         map[int]bool{},
		functionNames:         map[int]string{},
		messageOutputID:       -1,
		toolChoice:            toolChoice,
		traceID:               traceID,
		persistResponse:       persistResponse,
	}
}

func (s *responsesStreamRuntime) finalize() {
	finalThinking := s.thinking.String()
	finalText := cleanVisibleOutput(s.text.String(), s.stripReferenceMarkers)

	if s.bufferToolContent {
		s.processToolStreamEvents(flushToolSieve(&s.sieve, s.toolNames), true)
	}

	textParsed := toolcall.ParseStandaloneToolCallsDetailed(finalText, s.toolNames)
	detected := textParsed.Calls
	s.logToolPolicyRejections(textParsed)

	if len(detected) > 0 {
		s.toolCallsEmitted = true
		if !s.toolCallsDoneEmitted {
			s.emitFunctionCallDoneEvents(detected)
		}
	}

	s.closeMessageItem()

	if s.toolChoice.IsRequired() && len(detected) == 0 {
		s.failed = true
		message := "tool_choice requires at least one valid tool call."
		failedResp := map[string]any{
			"id":          s.responseID,
			"type":        "response",
			"object":      "response",
			"model":       s.model,
			"status":      "failed",
			"output":      []any{},
			"output_text": "",
			"error": map[string]any{
				"message": message,
				"type":    "invalid_request_error",
				"code":    "tool_choice_violation",
				"param":   nil,
			},
		}
		if s.persistResponse != nil {
			s.persistResponse(failedResp)
		}
		s.sendEvent("response.failed", openaifmt.BuildResponsesFailedPayload(s.responseID, s.model, message, "tool_choice_violation"))
		s.sendDone()
		return
	}
	s.closeIncompleteFunctionItems()

	obj := s.buildCompletedResponseObject(finalThinking, finalText, detected)
	if s.outputTokens > 0 {
		if usage, ok := obj["usage"].(map[string]any); ok {
			usage["output_tokens"] = s.outputTokens
		}
	}
	if s.promptTokens > 0 || s.outputTokens > 0 {
		if usage, ok := obj["usage"].(map[string]any); ok {
			if s.promptTokens > 0 {
				usage["input_tokens"] = s.promptTokens
			}
			if s.outputTokens > 0 {
				usage["output_tokens"] = s.outputTokens
			}
			input, _ := usage["input_tokens"].(int)
			output, _ := usage["output_tokens"].(int)
			usage["total_tokens"] = input + output
		}
	}
	if s.persistResponse != nil {
		s.persistResponse(obj)
	}
	s.sendEvent("response.completed", openaifmt.BuildResponsesCompletedPayload(obj))
	s.sendDone()
}

func (s *responsesStreamRuntime) logToolPolicyRejections(textParsed toolcall.ToolCallParseResult) {
	logRejected := func(parsed toolcall.ToolCallParseResult, channel string) {
		rejected := filteredRejectedToolNamesForLog(parsed.RejectedToolNames)
		if !parsed.RejectedByPolicy || len(rejected) == 0 {
			return
		}
		config.Logger.Warn(
			"[responses] rejected tool calls by policy",
			"trace_id", strings.TrimSpace(s.traceID),
			"channel", channel,
			"tool_choice_mode", s.toolChoice.Mode,
			"rejected_tool_names", strings.Join(rejected, ","),
		)
	}
	logRejected(textParsed, "text")
}

func (s *responsesStreamRuntime) onParsed(parsed sse.LineResult) streamengine.ParsedDecision {
	if !parsed.Parsed {
		return streamengine.ParsedDecision{}
	}
	if parsed.PromptTokens > 0 {
		s.promptTokens = parsed.PromptTokens
	}
	if parsed.OutputTokens > 0 {
		s.outputTokens = parsed.OutputTokens
	}
	if parsed.ContentFilter || parsed.ErrorMessage != "" || parsed.Stop {
		return streamengine.ParsedDecision{Stop: true}
	}

	contentSeen := false
	for _, p := range parsed.Parts {
		cleanedText := cleanVisibleOutput(p.Text, s.stripReferenceMarkers)
		if cleanedText == "" {
			continue
		}
		if p.Type != "thinking" && s.searchEnabled && sse.IsCitation(cleanedText) {
			continue
		}
		contentSeen = true
		if p.Type == "thinking" {
			if !s.thinkingEnabled {
				continue
			}
			trimmed := sse.TrimContinuationOverlap(s.thinking.String(), cleanedText)
			if trimmed == "" {
				continue
			}
			s.thinking.WriteString(trimmed)
			s.sendEvent("response.reasoning.delta", openaifmt.BuildResponsesReasoningDeltaPayload(s.responseID, trimmed))
			continue
		}

		trimmed := sse.TrimContinuationOverlap(s.text.String(), cleanedText)
		if trimmed == "" {
			continue
		}
		s.text.WriteString(trimmed)
		if !s.bufferToolContent {
			s.emitTextDelta(trimmed)
			continue
		}
		s.processToolStreamEvents(processToolSieveChunk(&s.sieve, trimmed, s.toolNames), true)
	}

	return streamengine.ParsedDecision{ContentSeen: contentSeen}
}
