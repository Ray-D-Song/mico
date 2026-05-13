package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/ray-d-song/mico/pkg/utils"
)

type ImageSaver struct {
	workDir string
}

func NewImageSaver(workDir string) *ImageSaver {
	return &ImageSaver{workDir: workDir}
}

func (s *ImageSaver) SaveOne(ctx context.Context, containerName, imageRef string) error {
	client := GetClient()
	reader, err := client.ImageSave(ctx, []string{imageRef})
	if err != nil {
		return fmt.Errorf("failed to save image %s: %w", imageRef, err)
	}
	defer reader.Close()

	utils.EnsureDir(utils.ServiceImageDir(s.workDir, containerName))

	imagePath := utils.ServiceImageTar(s.workDir, containerName)
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

func (s *ImageSaver) SaveBatch(ctx context.Context, items []struct {
	ContainerName string
	ImageRef     string
}, concurrent int) error {
	if len(items) == 0 {
		return nil
	}

	if concurrent <= 0 {
		concurrent = 1
	}

	type result struct {
		containerName string
		err          error
	}

	sem := make(chan struct{}, concurrent)
	results := make(chan result, len(items))
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		go func(cName, imgRef string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			err := s.SaveOne(ctx, cName, imgRef)
			results <- result{containerName: cName, err: err}
		}(item.ContainerName, item.ImageRef)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			return fmt.Errorf("failed to save image for %s: %w", r.containerName, r.err)
		}
	}

	return nil
}
