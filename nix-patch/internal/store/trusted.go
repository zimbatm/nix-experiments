package store

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// StoreInfo represents the output of 'nix store info --json'
type StoreInfo struct {
	Trusted int    `json:"trusted"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

// IsTrustedUser checks if the current user is a trusted user in the Nix store
func IsTrustedUser() (bool, error) {
	cmd := exec.Command("nix", "store", "info", "--json")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get store info: %w", err)
	}

	var info StoreInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return false, fmt.Errorf("failed to parse store info: %w", err)
	}

	return info.Trusted == 1, nil
}
