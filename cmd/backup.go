package cmd

import (
	"context"
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
	// 1. call packer.pack to create local archive
	// 2. send archive to s3 using s3 client
	// 3. delete local archive after successful upload
	// 4. call cleanupOldBackups to remove old backups if retention is set

	utils.PrintI("Packing containers...\n")
	packer.Pack(ctx, packer.PackOptions{
		OutputPath:    utils.GetS3TempZstdPath(),
		Containers:    containers,
		Incremental:   false,
		Concurrent:    concurrent,
		InspectConfig: inspectContainerConfig,
	})
	return nil
}

func cleanupOldBackups(ctx context.Context, bucketName string, keep int) error {
	// 1. list objects in bucket with prefix "backup-"
	// 2. sort by creation date, delete all but the most recent 'keep' objects

	return nil
}
