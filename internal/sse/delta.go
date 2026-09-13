package sse

import "strings"

// DeepSeek omits unchanged paths and operations; BATCH values have their own
// inherited path/operation, relative to the enclosing delta's path.
type deltaState struct {
	path string
	op   string
}

func (s *deltaState) expand(chunk map[string]any) []map[string]any {
	if _, ok := chunk["v"]; !ok {
		return []map[string]any{chunk}
	}
	if value, ok := chunk["v"].(map[string]any); ok {
		if _, snapshot := value["response"].(map[string]any); snapshot && chunk["p"] == nil {
			// Continuation snapshots also occur without a new ready event in
			// older streams; they restart the compressed delta sequence.
			s.path, s.op = "", "SET"
		}
	}
	if path, ok := chunk["p"].(string); ok {
		s.path = path
	}
	if op, ok := chunk["o"].(string); ok {
		s.op = op
	}
	if s.op == "" {
		s.op = "SET"
	}
	items, batch := chunk["v"].([]any)
	if s.op == "BATCH" && batch {
		var out []map[string]any
		var nested deltaState
		for _, item := range items {
			child, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, delta := range nested.expand(child) {
				path, _ := delta["p"].(string)
				if s.path != "" {
					delta["p"] = s.path + "/" + path
				}
				out = append(out, delta)
			}
		}
		return out
	}
	out := make(map[string]any, len(chunk)+2)
	for key, value := range chunk {
		out[key] = value
	}
	out["p"], out["o"] = s.path, s.op
	return []map[string]any{out}
}

type contentLineParser struct {
	delta deltaState
	event string
}

func (p *contentLineParser) parse(raw []byte, thinkingEnabled bool, currentType string) LineResult {
	line := strings.TrimSpace(string(raw))
	if strings.HasPrefix(line, "event:") {
		p.event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		if p.event == "ready" {
			p.delta = deltaState{}
		}
		return LineResult{NextType: currentType}
	}
	if line == "" {
		p.event = ""
	}
	if !strings.HasPrefix(line, "data:") {
		return LineResult{NextType: currentType}
	}
	event := p.event
	p.event = ""
	chunk, done, parsed := ParseDeepSeekSSELine(raw)
	result := LineResult{Parsed: parsed, Stop: done, NextType: currentType}
	if !parsed || done {
		return result
	}
	observeResponseMessageID(chunk, &result.ResponseMessageID)
	switch event {
	case "ready", "update_file", "update_session", "update_parent_message", "title", "hint", "toast", "debug", "close", "finish":
		// Named events carry metadata. In particular, close/finish can precede
		// an automatic continuation; response/status decides completion.
		return result
	}
	for _, delta := range p.delta.expand(chunk) {
		part := parseDeepSeekContentChunk(delta, thinkingEnabled, result.NextType)
		result.Parts = append(result.Parts, part.Parts...)
		result.ToolDetectionThinkingParts = append(result.ToolDetectionThinkingParts, part.ToolDetectionThinkingParts...)
		result.NextType = part.NextType
		result.Stop, result.ContentFilter, result.ErrorMessage = part.Stop, part.ContentFilter, part.ErrorMessage
		if part.ResponseMessageID > 0 {
			result.ResponseMessageID = part.ResponseMessageID
		}
		if part.Stop {
			break
		}
	}
	return result
}

func isSearchMetadataPath(path string) bool {
	path = strings.TrimPrefix(path, "response/")
	if path == "search_triggered" || path == "search_status" {
		return true
	}
	if !strings.HasPrefix(path, "fragments/") {
		return false
	}
	fields := strings.Split(path, "/")
	return len(fields) > 2 && fields[2] != "content"
}
