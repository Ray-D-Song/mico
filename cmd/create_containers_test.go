package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestCreateContainersReturnsRunErrors(t *testing.T) {
	oldRunContainer := runContainer
	oldForceRestore := forceRestore
	t.Cleanup(func() {
		runContainer = oldRunContainer
		forceRestore = oldForceRestore
	})

	runContainer = func(args ...string) ([]byte, error) {
		return []byte("docker run failed"), errors.New("exit 1")
	}
	forceRestore = false

	workDir := t.TempDir()
	writeCreateContainerFixture(t, workDir, "mico-bad")

	err := createContainers(workDir)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to create 1 container")
	require.Contains(t, err.Error(), "mico-bad")
	require.Contains(t, err.Error(), "docker run failed")
}

func writeCreateContainerFixture(t *testing.T, workDir, containerName string) {
	t.Helper()

	require.NoError(t, os.WriteFile(utils.ManifestPath(workDir), []byte(`{
  "version": "1.0",
  "services": [
    {
      "name": "bad",
      "container_name": "`+containerName+`",
      "image": "alpine:latest"
    }
  ]
}`), 0644))

	configDir := utils.ServiceConfigDir(workDir, containerName)
	require.NoError(t, os.MkdirAll(configDir, 0755))
	require.NoError(t, os.WriteFile(utils.ServiceConfigJSON(workDir, containerName), []byte(`{
  "Image": "alpine:latest"
}`), 0644))
	require.NoError(t, os.WriteFile(utils.ServiceHostJSON(workDir, containerName), []byte(`{}`), 0644))
}
