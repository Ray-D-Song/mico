package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	InitializeClient()
}

func TestNewScanner(t *testing.T) {
	scanner := NewScanner()
	assert.NotNil(t, scanner)
	assert.IsType(t, &Scanner{}, scanner)
}

func TestScanner_Functions_ReturnCorrectTypes(t *testing.T) {
	scanner := NewScanner()
	ctx := context.Background()

	// Test ScanRunningContainers returns correct type
	containers, err := scanner.ScanRunningContainers(ctx)
	// Should return slice and error (both can be nil/empty, that's fine)
	assert.NotNil(t, containers) // slice should not be nil (can be empty though)
	// Error can be nil or not nil depending on Docker availability
	_ = err

	// Test ScanAllContainers returns correct type
	allContainers, err := scanner.ScanAllContainers(ctx)
	assert.NotNil(t, allContainers)
	_ = err

	// Test FilterContainersByNames returns correct type
	filteredContainers, err := scanner.FilterContainersByNames(ctx, []string{"test"})
	assert.NotNil(t, filteredContainers)
	_ = err
}

func TestScanner_FilterContainersByNames_EmptyInput(t *testing.T) {
	scanner := NewScanner()
	ctx := context.Background()

	// Test with empty names list
	containers, err := scanner.FilterContainersByNames(ctx, []string{})
	assert.NotNil(t, containers)
	assert.Len(t, containers, 0) // Should return empty slice
	_ = err

	// Test with nil names list
	containers, err = scanner.FilterContainersByNames(ctx, nil)
	assert.NotNil(t, containers)
	assert.Len(t, containers, 0)
	_ = err
}
