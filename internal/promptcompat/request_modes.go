package promptcompat

import (
	"ds2api/internal/config"
	"ds2api/internal/util"
)

// ResolveRequestModes is shared by every protocol after adapter-specific mode
// fields have been normalized. Model aliases only supply compatibility defaults.
func ResolveRequestModes(req map[string]any, model string) (thinking, search bool) {
	defaultThinking, defaultSearch, _ := config.GetModelConfig(model)
	thinking = util.ResolveThinkingEnabled(req, defaultThinking)
	if config.IsNoThinkingModel(model) {
		thinking = false
	}
	return thinking, util.ResolveSearchEnabled(req, defaultSearch)
}

// PreserveModeOverrides keeps explicit options when a third-party protocol
// translator drops extension fields, including during Vercel stream preparation.
func PreserveModeOverrides(translated, original map[string]any) {
	if enabled, ok := util.ResolveThinkingOverride(original); ok {
		translated["thinking_enabled"] = enabled
	}
	if enabled, ok := util.ResolveSearchOverride(original); ok {
		translated["search_enabled"] = enabled
	}
}
