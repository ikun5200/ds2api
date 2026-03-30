package openai

func (h *Handler) toolcallFeatureMatchEnabled() bool {
	return true
}

func (h *Handler) toolcallEarlyEmitHighConfidence() bool {
	return true
}
