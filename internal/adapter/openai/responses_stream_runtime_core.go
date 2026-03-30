package openai

import (
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

	thinkingEnabled bool
	searchEnabled   bool

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
	toolNames []string,
	bufferToolContent bool,
	emitEarlyToolDeltas bool,
	toolChoice util.ToolChoicePolicy,
	traceID string,
	persistResponse func(obj map[string]any),
) *responsesStreamRuntime {
	return &responsesStreamRuntime{
		w:                   w,
		rc:                  rc,
		canFlush:            canFlush,
		responseID:          responseID,
		model:               model,
		finalPrompt:         finalPrompt,
		thinkingEnabled:     thinkingEnabled,
		searchEnabled:       searchEnabled,
		toolNames:           toolNames,
		bufferToolContent:   bufferToolContent,
		emitEarlyToolDeltas: emitEarlyToolDeltas,
		streamToolCallIDs:   map[int]string{},
		functionItemIDs:     map[int]string{},
		functionOutputIDs:   map[int]int{},
		functionArgs:        map[int]string{},
		functionDone:        map[int]bool{},
		functionAdded:       map[int]bool{},
		functionNames:       map[int]string{},
		messageOutputID:     -1,
		toolChoice:          toolChoice,
		traceID:             traceID,
		persistResponse:     persistResponse,
	}
}

func (s *responsesStreamRuntime) finalize() {
	finalThinking := s.thinking.String()
	finalText := sanitizeLeakedOutput(s.text.String())

	if s.bufferToolContent {
		s.processToolStreamEvents(flushToolSieve(&s.sieve, s.toolNames), true)
	}

	textParsed := util.ParseStandaloneToolCallsDetailed(finalText, s.toolNames)
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
	if s.persistResponse != nil {
		s.persistResponse(obj)
	}
	s.sendEvent("response.completed", openaifmt.BuildResponsesCompletedPayload(obj))
	s.sendDone()
}

func (s *responsesStreamRuntime) logToolPolicyRejections(textParsed util.ToolCallParseResult) {
	logRejected := func(parsed util.ToolCallParseResult, channel string) {
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
	if parsed.ContentFilter || parsed.ErrorMessage != "" || parsed.Stop {
		return streamengine.ParsedDecision{Stop: true}
	}

	contentSeen := false
	for _, p := range parsed.Parts {
		if p.Text == "" {
			continue
		}
		if p.Type != "thinking" && s.searchEnabled && sse.IsCitation(p.Text) {
			continue
		}
		contentSeen = true
		if p.Type == "thinking" {
			if !s.thinkingEnabled {
				continue
			}
			s.thinking.WriteString(p.Text)
			s.sendEvent("response.reasoning.delta", openaifmt.BuildResponsesReasoningDeltaPayload(s.responseID, p.Text))
			continue
		}

		cleanedText := sanitizeLeakedOutput(p.Text)
		if cleanedText == "" {
			continue
		}
		s.text.WriteString(cleanedText)
		if !s.bufferToolContent {
			s.emitTextDelta(cleanedText)
			continue
		}
		s.processToolStreamEvents(processToolSieveChunk(&s.sieve, cleanedText, s.toolNames), true)
	}

	return streamengine.ParsedDecision{ContentSeen: contentSeen}
}
