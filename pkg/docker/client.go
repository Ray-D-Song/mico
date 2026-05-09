package docker

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/docker/go-sdk/client"
	"github.com/ray-d-song/mico/pkg/runtime"
)

var (
	// Global singleton Docker client instance
	dockerClient *client.Client
	clientOnce   sync.Once
	clientErr    error
)

// InitializeClient initializes the global Docker client singleton.
// It detects the container runtime (Docker or Podman) and connects accordingly.
// This should be called once at the start of the application.
func InitializeClient() error {
	clientOnce.Do(func() {
		rt := runtime.Detect()
		if rt.Socket != "" {
			os.Setenv("DOCKER_HOST", "unix://"+rt.Socket)
		}

		cli, err := client.New(context.Background())
		if err != nil {
			hint := ""
			if rt.Type == runtime.Podman {
				if os.Geteuid() == 0 {
					hint = "\nEnable the Podman socket with: systemctl enable --now podman.socket"
				} else {
					hint = "\nEnable the rootless Podman socket with: systemctl --user enable --now podman.socket"
				}
			}
			clientErr = fmt.Errorf("failed to create %s client: %w%s", rt.Type, err, hint)
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
