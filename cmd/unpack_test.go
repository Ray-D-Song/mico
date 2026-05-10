package cmd

import (
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTarExtractsRegularFiles(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "archive.tar")
	destDir := filepath.Join(tempDir, "dest")

	writeTestTar(t, tarPath, map[string]string{
		"manifest.json":         `{"version":"1.0"}`,
		"mico-web/config/a.txt": "hello",
	})

	require.NoError(t, extractTar(tarPath, destDir))

	data, err := os.ReadFile(filepath.Join(destDir, "mico-web", "config", "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
}

func TestExtractTarRejectsPathTraversal(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "archive.tar")
	destDir := filepath.Join(tempDir, "dest")
	escapedPath := filepath.Join(tempDir, "evil.txt")

	writeTestTar(t, tarPath, map[string]string{
		"../evil.txt": "owned",
	})

	err := extractTar(tarPath, destDir)
	require.Error(t, err)
	require.NoFileExists(t, escapedPath)
}

func TestExtractTarRejectsAbsolutePath(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "archive.tar")
	destDir := filepath.Join(tempDir, "dest")

	writeTestTar(t, tarPath, map[string]string{
		"/tmp/mico-evil.txt": "owned",
	})

	require.Error(t, extractTar(tarPath, destDir))
}

func TestExtractTarRejectsLinks(t *testing.T) {
	tempDir := t.TempDir()
	tarPath := filepath.Join(tempDir, "archive.tar")
	destDir := filepath.Join(tempDir, "dest")

	f, err := os.Create(tarPath)
	require.NoError(t, err)
	tw := tar.NewWriter(f)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/tmp",
		Mode:     0777,
	}))
	require.NoError(t, tw.Close())
	require.NoError(t, f.Close())

	require.Error(t, extractTar(tarPath, destDir))
}

func writeTestTar(t *testing.T, tarPath string, files map[string]string) {
	t.Helper()

	f, err := os.Create(tarPath)
	require.NoError(t, err)

	tw := tar.NewWriter(f)

	for name, content := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, f.Close())
}
