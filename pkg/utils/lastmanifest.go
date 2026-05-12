package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ray-d-song/mico/pkg/core"
)

func LoadLastManifest() (*core.LastManifest, error) {
	path := GetLastManifestPath()
	if path == "" {
		return nil, fmt.Errorf("failed to get home directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read last manifest: %w", err)
	}

	var lm core.LastManifest
	if err := json.Unmarshal(data, &lm); err != nil {
		return nil, fmt.Errorf("failed to parse last manifest: %w", err)
	}

	return &lm, nil
}

func SaveLastManifest(pkgHash string, manifest core.PackageManifest) error {
	path := GetLastManifestPath()
	if path == "" {
		return fmt.Errorf("failed to get home directory")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	lm := core.LastManifest{
		PackageHash: pkgHash,
		Manifest:   manifest,
	}

	data, err := json.MarshalIndent(lm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal last manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write last manifest: %w", err)
	}

	return nil
}
