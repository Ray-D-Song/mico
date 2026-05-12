package utils

import (
	"os"
	"path/filepath"
)

func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mico")
}

func GetLastManifestPath() string {
	dir := GetConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "last_manifest.json")
}

func GetS3ConfigPath() string {
	dir := GetConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "s3.ini")
}
