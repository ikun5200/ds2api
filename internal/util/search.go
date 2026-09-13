package util

// ResolveSearchEnabled applies an explicit request override before the model's
// legacy default. A false value must be distinguished from an absent setting.
func ResolveSearchEnabled(req map[string]any, defaultEnabled bool) bool {
	if enabled, ok := ResolveSearchOverride(req); ok {
		return enabled
	}
	return defaultEnabled
}

func ResolveSearchOverride(req map[string]any) (bool, bool) {
	for _, key := range []string{"search_enabled", "search"} {
		if enabled, ok := parseThinkingSetting(req[key]); ok {
			return enabled, true
		}
	}
	if extra, ok := req["extra_body"].(map[string]any); ok {
		for _, key := range []string{"search_enabled", "search"} {
			if enabled, ok := parseThinkingSetting(extra[key]); ok {
				return enabled, true
			}
		}
	}
	return false, false
}
