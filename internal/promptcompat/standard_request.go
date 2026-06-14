package promptcompat

import (
	"strings"

	"ds2api/internal/config"
	"ds2api/internal/prompt"
)

type StandardRequest struct {
	Surface                 string
	RequestedModel          string
	ResolvedModel           string
	ResponseModel           string
	Messages                []any
	HistoryText             string
	PromptTokenText         string
	CurrentInputFileApplied bool
	CurrentInputFileID      string
	CurrentToolsFileID      string
	ToolsRaw                any
	FinalPrompt             string
	PromptPrepareOptions    prompt.PrepareOptions
	PromptPrepareOptionsSet bool
	ToolNames               []string
	ToolChoice              ToolChoicePolicy
	Stream                  bool
	Thinking                bool
	Search                  bool
	RefFileIDs              []string
	RefFileTokens           int
	PassThrough             map[string]any
}

type PromptPrepareOptionsReader interface {
	OutputIntegrityGuardEnabled() bool
	OutputIntegrityGuardPrompt() string
}

func PromptPrepareOptionsFromConfig(store any) prompt.PrepareOptions {
	opts := prompt.DefaultPrepareOptions()
	reader, ok := store.(PromptPrepareOptionsReader)
	if !ok || reader == nil {
		return opts
	}
	opts.OutputIntegrityGuardEnabled = reader.OutputIntegrityGuardEnabled()
	if customPrompt := strings.TrimSpace(reader.OutputIntegrityGuardPrompt()); customPrompt != "" {
		opts.OutputIntegrityGuardPrompt = customPrompt
	}
	return opts
}

func (r StandardRequest) PrepareOptionsOrDefault() prompt.PrepareOptions {
	if !r.PromptPrepareOptionsSet {
		return prompt.DefaultPrepareOptions()
	}
	opts := r.PromptPrepareOptions
	if strings.TrimSpace(opts.OutputIntegrityGuardPrompt) == "" {
		opts.OutputIntegrityGuardPrompt = prompt.DefaultOutputIntegrityGuardPrompt
	}
	return opts
}

type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceForced   ToolChoiceMode = "forced"
)

type ToolChoicePolicy struct {
	Mode       ToolChoiceMode
	ForcedName string
	Allowed    map[string]struct{}
}

func DefaultToolChoicePolicy() ToolChoicePolicy {
	return ToolChoicePolicy{Mode: ToolChoiceAuto}
}

func (p ToolChoicePolicy) IsNone() bool {
	return p.Mode == ToolChoiceNone
}

func (p ToolChoicePolicy) IsRequired() bool {
	return p.Mode == ToolChoiceRequired || p.Mode == ToolChoiceForced
}

func (p ToolChoicePolicy) Allows(name string) bool {
	if len(p.Allowed) == 0 {
		return true
	}
	_, ok := p.Allowed[name]
	return ok
}

func (r StandardRequest) CompletionPayload(sessionID string) map[string]any {
	modelID := r.ResolvedModel
	if modelID == "" {
		modelID = r.RequestedModel
	}
	modelType := "default"
	if resolvedType, ok := config.GetModelType(modelID); ok {
		modelType = resolvedType
	}
	refFileIDs := make([]any, 0, len(r.RefFileIDs))
	for _, fileID := range r.RefFileIDs {
		if fileID == "" {
			continue
		}
		refFileIDs = append(refFileIDs, fileID)
	}
	payload := map[string]any{
		"chat_session_id":   sessionID,
		"model_type":        modelType,
		"parent_message_id": nil,
		"prompt":            r.FinalPrompt,
		"ref_file_ids":      refFileIDs,
		"thinking_enabled":  r.Thinking,
		"search_enabled":    r.Search,
	}
	for k, v := range r.PassThrough {
		payload[k] = v
	}
	return payload
}
