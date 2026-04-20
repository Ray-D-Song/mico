package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
)

type Inspector struct {
	workDir string
}

func NewInspector(workDir string) *Inspector {
	return &Inspector{workDir: workDir}
}

func (i *Inspector) InspectOne(ctx context.Context, name string) (*container.Config, error) {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	configPath := filepath.Join(i.workDir, "configs", name+".json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := json.MarshalIndent(resp.Config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	return resp.Config, nil
}

func (i *Inspector) InspectBatch(ctx context.Context, names []string, concurrent int) (map[string]*container.Config, error) {
	if len(names) == 0 {
		return nil, nil
	}

	if concurrent <= 0 {
		concurrent = 1
	}

	type result struct {
		name string
		cfg  *container.Config
		err  error
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
			results <- result{name: n, cfg: cfg, err: err}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	configs := make(map[string]*container.Config)
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", r.name, r.err)
		}
		configs[r.name] = r.cfg
	}

	return configs, nil
}
