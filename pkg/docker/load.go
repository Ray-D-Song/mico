package docker

import (
	"fmt"
	"os/exec"

	"github.com/ray-d-song/migo/pkg/utils"
)

func LoadImage(imagePath string) error {
	if !utils.FileExists(imagePath) {
		return fmt.Errorf("image file not found: %s", imagePath)
	}

	cmd := exec.Command("docker", "load", "-i", imagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load image: %w, output: %s", err, string(output))
	}

	return nil
}