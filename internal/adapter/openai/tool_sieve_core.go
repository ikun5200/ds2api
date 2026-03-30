package openai

import (
	"strings"

	"ds2api/internal/util"
)

func processToolSieveChunk(state *toolStreamSieveState, chunk string, toolNames []string) []toolStreamEvent {
	if state == nil {
		return nil
	}
	if chunk != "" {
		state.pending.WriteString(chunk)
	}
	events := make([]toolStreamEvent, 0, 2)
	if len(state.pendingToolCalls) > 0 {
		events = append(events, toolStreamEvent{ToolCalls: state.pendingToolCalls})
		state.pendingToolRaw = ""
		state.pendingToolCalls = nil
	}

	for {
		if state.capturing {
			if state.pending.Len() > 0 {
				state.capture.WriteString(state.pending.String())
				state.pending.Reset()
			}
			prefix, calls, suffix, ready := consumeToolCapture(state, toolNames)
			if !ready {
				break
			}
			captured := state.capture.String()
			state.capture.Reset()
			state.capturing = false
			state.resetIncrementalToolState()
			if len(calls) > 0 {
				if prefix != "" {
					state.noteText(prefix)
					events = append(events, toolStreamEvent{Content: prefix})
				}
				if suffix != "" {
					state.pending.WriteString(suffix)
				}
				_ = captured
				state.pendingToolCalls = calls
				continue
			}
			if prefix != "" {
				state.noteText(prefix)
				events = append(events, toolStreamEvent{Content: prefix})
			}
			if suffix != "" {
				state.pending.WriteString(suffix)
			}
			continue
		}

		pending := state.pending.String()
		if pending == "" {
			break
		}
		start := findToolSegmentStart(pending)
		if start >= 0 {
			prefix := pending[:start]
			if prefix != "" {
				state.noteText(prefix)
				events = append(events, toolStreamEvent{Content: prefix})
			}
			state.pending.Reset()
			state.capture.WriteString(pending[start:])
			state.capturing = true
			state.resetIncrementalToolState()
			continue
		}

		safe, hold := splitSafeContentForToolDetection(pending)
		if safe == "" {
			break
		}
		state.pending.Reset()
		state.pending.WriteString(hold)
		state.noteText(safe)
		events = append(events, toolStreamEvent{Content: safe})
	}

	return events
}

func flushToolSieve(state *toolStreamSieveState, toolNames []string) []toolStreamEvent {
	if state == nil {
		return nil
	}
	events := processToolSieveChunk(state, "", toolNames)
	if len(state.pendingToolCalls) > 0 {
		events = append(events, toolStreamEvent{ToolCalls: state.pendingToolCalls})
		state.pendingToolRaw = ""
		state.pendingToolCalls = nil
	}
	if state.capturing {
		consumedPrefix, consumedCalls, consumedSuffix, ready := consumeToolCapture(state, toolNames)
		if ready {
			if consumedPrefix != "" {
				state.noteText(consumedPrefix)
				events = append(events, toolStreamEvent{Content: consumedPrefix})
			}
			if len(consumedCalls) > 0 {
				events = append(events, toolStreamEvent{ToolCalls: consumedCalls})
			}
			if consumedSuffix != "" {
				state.noteText(consumedSuffix)
				events = append(events, toolStreamEvent{Content: consumedSuffix})
			}
		} else {
			content := state.capture.String()
			if content != "" {
				// If the captured text looks like an incomplete XML tool call block,
				// swallow it to prevent leaking raw XML tags to the client.
				if hasOpenXMLToolTag(content) {
					// Drop it silently — incomplete tool call.
				} else {
					state.noteText(content)
					events = append(events, toolStreamEvent{Content: content})
				}
			}
		}
		state.capture.Reset()
		state.capturing = false
		state.resetIncrementalToolState()
	}
	if state.pending.Len() > 0 {
		content := state.pending.String()
		// Safety: if pending contains XML tool tag fragments (e.g. "tool_calls>"
		// from a split closing tag), swallow them instead of leaking.
		if hasOpenXMLToolTag(content) || looksLikeXMLToolTagFragment(content) {
			// Drop it — likely an incomplete tool call fragment.
		} else {
			state.noteText(content)
			events = append(events, toolStreamEvent{Content: content})
		}
		state.pending.Reset()
	}
	return events
}

func splitSafeContentForToolDetection(s string) (safe, hold string) {
	if s == "" {
		return "", ""
	}
	suspiciousStart := findSuspiciousPrefixStart(s)
	if suspiciousStart < 0 {
		return s, ""
	}
	if suspiciousStart > 0 {
		return s[:suspiciousStart], s[suspiciousStart:]
	}
	// If suspicious content starts at position 0, keep holding until we can
	// parse a complete tool JSON block or reach stream flush.
	return "", s
}

func findSuspiciousPrefixStart(s string) int {
	start := -1
	indices := []int{
		strings.LastIndex(s, "{"),
		strings.LastIndex(s, "["),
		strings.LastIndex(s, "```"),
	}
	for _, idx := range indices {
		if idx > start {
			start = idx
		}
	}
	// Also check for partial XML tool tag at end of string.
	if xmlIdx := findPartialXMLToolTagStart(s); xmlIdx >= 0 && xmlIdx > start {
		start = xmlIdx
	}
	return start
}

func findToolSegmentStart(s string) int {
	if s == "" {
		return -1
	}
	lower := strings.ToLower(s)
	keywords := []string{"tool_calls", "\"function\"", "function.name:"}
	bestKeyIdx := -1
	for _, kw := range keywords {
		idx := strings.Index(lower, kw)
		if idx >= 0 && (bestKeyIdx < 0 || idx < bestKeyIdx) {
			bestKeyIdx = idx
		}
	}
	// Also detect XML tool call tags.
	for _, tag := range xmlToolTagsToDetect {
		idx := strings.Index(lower, tag)
		if idx >= 0 && (bestKeyIdx < 0 || idx < bestKeyIdx) {
			bestKeyIdx = idx
		}
	}
	if bestKeyIdx < 0 {
		return -1
	}
	// For XML tags, the '<' is itself the segment start.
	if bestKeyIdx < len(s) && s[bestKeyIdx] == '<' {
		if fenceStart, ok := openFenceStartBefore(s, bestKeyIdx); ok {
			return fenceStart
		}
		return bestKeyIdx
	}
	start := strings.LastIndex(s[:bestKeyIdx], "{")
	if start < 0 {
		start = bestKeyIdx
	}
	// If the keyword matched inside an XML tag (e.g. "tool_calls" in "<tool_calls>"),
	// back up past the '<' to capture the full tag.
	if start > 0 && s[start-1] == '<' {
		start--
	}
	if fenceStart, ok := openFenceStartBefore(s, start); ok {
		return fenceStart
	}
	return start
}

func consumeToolCapture(state *toolStreamSieveState, toolNames []string) (prefix string, calls []util.ParsedToolCall, suffix string, ready bool) {
	captured := state.capture.String()
	if captured == "" {
		return "", nil, "", false
	}

	// Try XML tool call extraction first.
	if xmlPrefix, xmlCalls, xmlSuffix, xmlReady := consumeXMLToolCapture(captured, toolNames); xmlReady {
		return xmlPrefix, xmlCalls, xmlSuffix, true
	}
	// If XML tags are present but block is incomplete, keep buffering.
	if hasOpenXMLToolTag(captured) {
		return "", nil, "", false
	}

	lower := strings.ToLower(captured)
	keyIdx := -1
	keywords := []string{"tool_calls", "\"function\"", "function.name:"}
	for _, kw := range keywords {
		idx := strings.Index(lower, kw)
		if idx >= 0 && (keyIdx < 0 || idx < keyIdx) {
			keyIdx = idx
		}
	}

	if keyIdx < 0 {
		return "", nil, "", false
	}
	start := strings.LastIndex(captured[:keyIdx], "{")
	if start < 0 {
		start = keyIdx
	}
	obj, end, ok := extractJSONObjectFrom(captured, start)
	if !ok {
		return "", nil, "", false
	}
	prefixPart := captured[:start]
	suffixPart := captured[end:]
	parsed := util.ParseStandaloneToolCallsDetailed(obj, toolNames)
	if len(parsed.Calls) == 0 {
		if parsed.SawToolCallSyntax && parsed.RejectedByPolicy {
			// Parsed as tool-call payload but rejected by schema/policy:
			// consume it to avoid leaking raw tool_calls JSON to user content.
			return prefixPart, nil, suffixPart, true
		}
		// If it has obvious keywords but failed to parse even after loose repair,
		// we still might want to intercept it if it looks like an attempt at tool call.
		// For now, keep the original logic but rely on loose JSON repair.
		return captured, nil, "", true
	}
	prefixPart, suffixPart = trimWrappingJSONFence(prefixPart, suffixPart)
	return prefixPart, parsed.Calls, suffixPart, true
}
