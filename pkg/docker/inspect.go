package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/ray-d-song/migo/pkg/utils"
)

type Inspector struct {
	workDir string
}

type MountInfo struct {
	Type        string `json:"type"`
	Source     string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly   bool   `json:"read_only"`
}

type ContainerMounts struct {
	ContainerName string     `json:"container_name"`
	Mounts       []MountInfo `json:"mounts"`
}

func NewInspector(workDir string) *Inspector {
	return &Inspector{workDir: workDir}
}

func (i *Inspector) InspectOne(ctx context.Context, containerName string) (*container.Config, error) {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	servicePath := filepath.Join(i.workDir, containerName)
	utils.EnsureDir(servicePath + "/config")

	configPath := filepath.Join(servicePath, "config", "config.json")
	data, err := json.MarshalIndent(resp.Config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	return resp.Config, nil
}

func (i *Inspector) SaveMounts(ctx context.Context, containerName string) (*ContainerMounts, error) {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	mounts := make([]MountInfo, 0, len(resp.Mounts))
	for _, m := range resp.Mounts {
		mounts = append(mounts, MountInfo{
			Type:        string(m.Type),
			Source:     m.Source,
			Destination: m.Destination,
			ReadOnly:   m.RW == false,
		})
	}

	containerMounts := &ContainerMounts{
		ContainerName: containerName,
		Mounts:       mounts,
	}

	servicePath := filepath.Join(i.workDir, containerName)
	utils.EnsureDir(servicePath + "/config")

	mountsPath := filepath.Join(servicePath, "config", "mounts.json")
	data, err := json.MarshalIndent(containerMounts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal mounts: %w", err)
	}

	if err := os.WriteFile(mountsPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write mounts file: %w", err)
	}

	return containerMounts, nil
}

func (i *Inspector) SaveHostConfig(ctx context.Context, containerName string) error {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	servicePath := filepath.Join(i.workDir, containerName)
	utils.EnsureDir(servicePath + "/config")

	hostPath := filepath.Join(servicePath, "config", "host.json")
	data, err := json.MarshalIndent(resp.HostConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal host config: %w", err)
	}

	if err := os.WriteFile(hostPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write host config file: %w", err)
	}

	return nil
}

func (i *Inspector) SaveNetworkSettings(ctx context.Context, containerName string) error {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	servicePath := filepath.Join(i.workDir, containerName)
	utils.EnsureDir(servicePath + "/config")

	networkPath := filepath.Join(servicePath, "config", "network.json")
	data, err := json.MarshalIndent(resp.NetworkSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal network settings: %w", err)
	}

	if err := os.WriteFile(networkPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write network settings file: %w", err)
	}

	return nil
}

func (i *Inspector) InspectBatch(ctx context.Context, names []string, concurrent int) (map[string]*container.Config, error) {
	if len(names) == 0 {
		return nil, nil
	}

	if concurrent <= 0 {
		concurrent = 1
	}

	type result struct {
		containerName string
		cfg          *container.Config
		err          error
	}

	sem := make(chan struct{}, concurrent)
	results := make(chan result, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			cfg, err := i.InspectOne(ctx, n)
			results <- result{containerName: n, cfg: cfg, err: err}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	configs := make(map[string]*container.Config)
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", r.containerName, r.err)
		}
		configs[r.containerName] = r.cfg
	}

	return configs, nil
}
