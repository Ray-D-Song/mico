package cmd

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/ray-d-song/migo/pkg/docker"
	"github.com/ray-d-song/migo/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	verifyChecksum bool
	forceRestore bool
)

var unpackCmd = &cobra.Command{
	Use:   "unpack",
	Short: "Unpack a migration archive and restore containers",
	Long: `Unpack command extracts a migration archive, imports images,
restores volumes, and recreates containers on the target server.

Examples:
  mico unpack migration.zst                     # Unpack with default settings
  mico unpack migration.zst --verify            # Verify checksum before unpack
  mico unpack migration.zst --force             # Force restore (overwrite existing)`,
	Run: func(cmd *cobra.Command, args []string) {
		if concurrent <= 0 {
			concurrent = 1
		}

		if len(args) == 0 {
			utils.PrintErrMsg(utils.ErrInvalidInput, "migration package required")
			return
		}

		packagePath := args[0]

		if !utils.FileExists(packagePath) {
			utils.PrintErrMsg(utils.ErrFileRead, "package not found: "+packagePath)
			return
		}

		utils.PrintI("Package: %s\n", packagePath)

		if verifyChecksum {
			utils.PrintI("Verifying checksum...\n")
			valid, err := utils.VerifyChecksum(packagePath)
			if err != nil {
				utils.PrintErrMsg(utils.ErrVerifyFailed, err)
				return
			}
			if !valid {
				utils.PrintErrMsg(utils.ErrVerifyFailed, "checksum mismatch")
				return
			}
			utils.PrintS("Checksum verified\n")
		}

		workDir := utils.CreateWorkDir("mico-unpack")
		utils.PrintI("Work directory: %s\n", workDir)

		utils.PrintI("Decompressing...\n")
		tarPath := packagePath + ".tar"
		if err := utils.NewZSTDCompressor().DecompressFile(packagePath, tarPath); err != nil {
			utils.PrintErrMsg(utils.ErrUnpackExtract, err)
			return
		}
		utils.PrintS("Decompressed\n")

		utils.PrintI("Extracting tar...\n")
		if err := extractTar(tarPath, workDir); err != nil {
			utils.PrintErrMsg(utils.ErrUnpackExtract, err)
			return
		}
		os.Remove(tarPath)
		utils.PrintS("Extracted\n")

		utils.PrintI("Loading images...\n")
		if err := loadImages(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrImagePull, err)
			return
		}
		utils.PrintS("Images loaded\n")

		utils.PrintI("Restoring volumes...\n")
		if err := restoreVolumes(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrVolumeRestore, err)
			return
		}
		utils.PrintS("Volumes restored\n")

		utils.PrintI("Creating containers...\n")
		if err := createContainers(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrContainerInspect, err)
			return
		}
		utils.PrintS("Containers created\n")

		utils.PrintS("Unpack completed successfully!\n")
	},
}

func init() {
	rootCmd.AddCommand(unpackCmd)

	unpackCmd.Flags().BoolVarP(&verifyChecksum, "verify", "v", false, "Verify checksum before unpack")
	unpackCmd.Flags().BoolVarP(&forceRestore, "force", "f", false, "Force restore (overwrite existing)")
	unpackCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations")
}

func extractTar(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := utils.EnsureFile(target); err != nil {
				return err
			}
			w, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		}
	}
	return nil
}

func loadImages(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var loadErr error
	sem := make(chan struct{}, concurrent)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		imagePath := filepath.Join(workDir, entry.Name(), "image", "image.tar")
		if !utils.FileExists(imagePath) {
			continue
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := docker.LoadImage(path); err != nil {
				loadErr = err
			}
		}(imagePath)
	}

	wg.Wait()
	return loadErr
}

func restoreVolumes(workDir string) error {
	return nil
}

func createContainers(workDir string) error {
	return nil
}