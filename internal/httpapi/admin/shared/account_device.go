package shared

import (
	"fmt"
	"strings"
)

// AccountDeviceIDOptional keeps an omitted device unchanged during metadata
// updates; an explicit empty string clears it. Device IDs are opaque strings.
func AccountDeviceIDOptional(m map[string]any) (string, bool, error) {
	raw, ok := m["device_id"]
	if !ok || raw == nil {
		return "", false, nil
	}
	deviceID, ok := raw.(string)
	if !ok {
		return "", false, fmt.Errorf("device_id must be a string")
	}
	return strings.TrimSpace(deviceID), true, nil
}

func ValidateAccountDeviceIDs(raw any) error {
	accounts, _ := raw.([]any)
	for i, rawAccount := range accounts {
		account, _ := rawAccount.(map[string]any)
		if _, _, err := AccountDeviceIDOptional(account); err != nil {
			return fmt.Errorf("accounts[%d].%w", i, err)
		}
	}
	return nil
}
