package store

import (
	"encoding/json"
	"fmt"
)

// StoreInfo represents the output of 'nix store info --json'
type StoreInfo struct {
	Trusted int    `json:"trusted"`
	URL     string `json:"url"`
	Version string `json:"version"`
}

// GetStoreInfo returns store information
func (s *Store) GetStoreInfo() (*StoreInfo, error) {
	output, err := s.execNix("store", "info", "--json")
	if err != nil {
		return nil, fmt.Errorf("failed to get store info: %w", err)
	}

	var info StoreInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse store info: %w", err)
	}

	return &info, nil
}
