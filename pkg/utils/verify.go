package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func SHA256File(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func SHA256String(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func GenerateChecksumFile(filePath string) (string, error) {
	hash, err := SHA256File(filePath)
	if err != nil {
		return "", err
	}

	checksumPath := filePath + ".sha256"
	checksumContent := hash + "  " + filepath.Base(filePath) + "\n"

	if err := os.WriteFile(checksumPath, []byte(checksumContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write checksum file: %w", err)
	}

	return hash, nil
}

func VerifyChecksum(filePath string) (bool, error) {
	checksumPath := filePath + ".sha256"

	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return false, fmt.Errorf("failed to read checksum file: %w", err)
	}

	var expectedHash string
	_, err = fmt.Sscanf(string(data), "%s", &expectedHash)
	if err != nil {
		return false, fmt.Errorf("failed to parse checksum file: %w", err)
	}

	actualHash, err := SHA256File(filePath)
	if err != nil {
		return false, err
	}

	return expectedHash == actualHash, nil
}
