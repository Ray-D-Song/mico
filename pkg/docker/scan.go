package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
)

// Scanner handles Docker container scanning operations
type Scanner struct{}

// NewScanner creates a new Docker scanner instance
func NewScanner() *Scanner {
	return &Scanner{}
}

// ScanRunningContainers scans and returns all currently running containers
func (s *Scanner) ScanRunningContainers(ctx context.Context) ([]container.Summary, error) {
	client := GetClient()

	// List only running containers
	containers, err := client.ContainerList(ctx, container.ListOptions{
		All: false, // Only running containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// ScanAllContainers scans and returns all containers (running and stopped)
func (s *Scanner) ScanAllContainers(ctx context.Context) ([]container.Summary, error) {
	client := GetClient()

	// List all containers including stopped ones
	containers, err := client.ContainerList(ctx, container.ListOptions{
		All: true, // Include stopped containers
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// FilterContainersByNames filters containers by specified names
func (s *Scanner) FilterContainersByNames(ctx context.Context, names []string) ([]container.Summary, error) {
	allContainers, err := s.ScanRunningContainers(ctx)
	if err != nil {
		return nil, err
	}

	// Create a map for quick lookup
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	filtered := []container.Summary{}
	for _, containerItem := range allContainers {
		// Check all names of the container (Docker containers can have multiple names)
		for _, containerName := range containerItem.Names {
			// Remove leading slash from container name
			cleanName := containerName
			if len(cleanName) > 0 && cleanName[0] == '/' {
				cleanName = cleanName[1:]
			}

			if nameMap[cleanName] {
				filtered = append(filtered, containerItem)
				break // Found a match, no need to check other names
			}
		}
	}

	return filtered, nil
}
