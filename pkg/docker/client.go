package docker

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/go-sdk/client"
)

var (
	// Global singleton Docker client instance
	dockerClient *client.Client
	clientOnce   sync.Once
	clientErr    error
)

// InitializeClient initializes the global Docker client singleton
// This should be called once at the start of the application
func InitializeClient() error {
	clientOnce.Do(func() {
		cli, err := client.New(context.Background())
		if err != nil {
			clientErr = fmt.Errorf("failed to create Docker client: %w", err)
			return
		}
		dockerClient = cli
	})
	return clientErr
}

// GetClient returns the global Docker client instance
// Panics if the client hasn't been initialized or initialization failed
func GetClient() *client.Client {
	if dockerClient == nil {
		panic("Docker client not initialized. Call InitializeClient() first.")
	}
	return dockerClient
}

// CloseClient closes the global Docker client connection
func CloseClient() error {
	if dockerClient != nil {
		return dockerClient.Close()
	}
	return nil
}
