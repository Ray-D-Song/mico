package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadImagesReturnsLoaderError(t *testing.T) {
	oldLoadImage := loadImage
	oldConcurrent := concurrent
	t.Cleanup(func() {
		loadImage = oldLoadImage
		concurrent = oldConcurrent
	})

	expectedErr := errors.New("load failed")
	loadImage = func(path string) error {
		if filepath.Base(filepath.Dir(filepath.Dir(path))) == "bad" {
			return expectedErr
		}
		return nil
	}
	concurrent = 4

	workDir := t.TempDir()
	for _, name := range []string{"good-1", "bad", "good-2"} {
		imageDir := filepath.Join(workDir, name, "image")
		require.NoError(t, os.MkdirAll(imageDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(imageDir, "image.tar"), []byte("fake image"), 0644))
	}

	require.ErrorIs(t, loadImages(workDir), expectedErr)
}
