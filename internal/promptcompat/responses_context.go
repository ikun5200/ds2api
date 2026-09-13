package promptcompat

import (
	"fmt"
	"strings"
)

// The response cache contains rendered output, not a replayable conversation.
// Reject stateful continuation instead of silently dropping previous images,
// instructions, and tool history while returning an apparently valid reply.
func ValidateResponsesContext(req map[string]any) error {
	value, exists := req["previous_response_id"]
	if !exists || value == nil {
		return nil
	}
	if id, ok := value.(string); ok && strings.TrimSpace(id) == "" {
		return nil
	}
	return fmt.Errorf("previous_response_id is not supported; include the full conversation and file references in input")
}
