package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type ImageSaver struct {
	workDir string
}

func NewImageSaver(workDir string) *ImageSaver {
	return &ImageSaver{workDir: workDir}
}

func (s *ImageSaver) SaveOne(ctx context.Context, name string) error {
	client := GetClient()
	reader, err := client.ImageSave(ctx, []string{name})
	if err != nil {
		return fmt.Errorf("failed to save image %s: %w", name, err)
	}
	defer reader.Close()

	imagePath := filepath.Join(s.workDir, "images", name+".tar")
	if err := os.MkdirAll(filepath.Dir(imagePath), 0755); err != nil {
		return fmt.Errorf("failed to create image dir: %w", err)
	}

	f, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("failed to write image: %w", err)
	}

	return nil
}

func (s *ImageSaver) SaveBatch(ctx context.Context, names []string, concurrent int) error {
	if len(names) == 0 {
		return nil
	}

	if concurrent <= 0 {
		concurrent = 1
	}

	type result struct {
		name string
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

			err := s.SaveOne(ctx, n)
			results <- result{name: n, err: err}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			return fmt.Errorf("failed to save image %s: %w", r.name, r.err)
		}
	}

	return nil
}

