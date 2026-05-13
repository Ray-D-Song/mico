package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/ray-d-song/mico/pkg/packer"
	"github.com/ray-d-song/mico/pkg/s3"
	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	bucket    string
	interval  int
	retention int
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Pack containers and upload to S3",
	Long: `Backup command packs running Docker containers and uploads the archive
to S3-compatible object storage. Supports periodic backup with --interval.

Examples:
  mico backup -b my-bucket                  # One-time backup
  mico backup -b my-bucket -c web,db        # Specific containers only
  mico backup -b my-bucket -i 24            # Every 24 hours
  mico backup -b my-bucket -i 6 -r 28       # Every 6 hours, keep last 28`,
	Run: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)

	backupCmd.Flags().StringVarP(&bucket, "bucket", "b", "", "S3 bucket name (default: from ~/.mico/s3.ini or mico-backups)")
	backupCmd.Flags().StringVarP(&containers, "containers", "c", "", "Comma-separated list of container names to pack (default: all running containers)")
	backupCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations (default: 1)")
	backupCmd.Flags().IntVarP(&interval, "interval", "i", 0, "Backup interval in hours (0 = run once)")
	backupCmd.Flags().IntVarP(&retention, "retention", "r", 0, "Number of backups to keep (0 = keep all)")
}

func runBackup(cmd *cobra.Command, args []string) {
	utils.SetSilent()

	if err := s3.InitializeClient(); err != nil {
		utils.PrintE("%s\n", err.Error())
		return
	}

	bucketName := resolveBucket()
	if bucketName == "" {
		utils.PrintE("no s3 bucket specified")
		return
	}
	ctx := cmd.Context()
	for {
		if err := doBackup(ctx, bucketName, containers); err != nil {
			utils.PrintE("%s\n", err.Error())
		}

		if interval <= 0 {
			break
		}
		time.Sleep(time.Duration(interval) * time.Hour)
	}
}

func resolveBucket() string {
	if bucket != "" {
		return bucket
	}
	if cfg := s3.GetConfig(); cfg.Bucket != "" {
		return cfg.Bucket
	}
	return "mico-backups"
}

func doBackup(ctx context.Context, bucketName string, containers string) error {
	utils.PrintI("Packing containers...\n")
	// Generate temporary zstd file
	outputPath := utils.GetS3TempZstdPath()
	if err := packer.Pack(ctx, packer.PackOptions{
		OutputPath:    outputPath,
		Containers:    containers,
		Incremental:   false,
		Concurrent:    concurrent,
		InspectConfig: inspectContainerConfig,
	}); err != nil {
		return fmt.Errorf("pack failed: %w", err)
	}

	utils.PrintI("Uploading to S3...\n")

	now := time.Now()
	baseName := filepath.Base(outputPath)
	packKey := "backup-" + now.Format("2006/01/02/150405") + "/" + baseName
	if err := s3.UploadFile(ctx, bucketName, packKey, outputPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	utils.PrintS("Uploaded: %s\n", packKey)

	checksumPath := outputPath + ".sha256"
	if utils.FileExists(checksumPath) {
		checksumKey := packKey + ".sha256"
		if err := s3.UploadFile(ctx, bucketName, checksumKey, checksumPath); err != nil {
			utils.PrintW("failed to upload checksum: %v\n", err)
		}
	}

	if err := os.Remove(outputPath); err != nil {
		utils.PrintW("failed to remove temp file: %v\n", err)
	}
	if err := os.Remove(checksumPath); err != nil && !os.IsNotExist(err) {
		utils.PrintW("failed to remove temp checksum: %v\n", err)
	}

	// Clean up old backups if retention is set
	if retention > 0 {
		utils.PrintI("Cleaning up old backups...\n")
		if err := cleanupOldBackups(ctx, bucketName, retention); err != nil {
			utils.PrintW("cleanup failed: %v\n", err)
		}
	}

	return nil
}

func cleanupOldBackups(ctx context.Context, bucketName string, keep int) error {
	objects, err := s3.ListObjects(ctx, bucketName, "backup-", 1000)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}
	if len(objects) <= keep {
		utils.PrintI("No old backups to clean up\n")
		return nil
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})

	for _, obj := range objects[keep:] {
		if err := s3.DeleteObject(ctx, bucketName, obj.Key); err != nil {
			return fmt.Errorf("failed to delete %s: %w", obj.Key, err)
		}
	}

	utils.PrintI("Cleanup completed\n")
	return nil
}
