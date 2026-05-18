package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

var (
	deleteObjectFn            = s3.DeleteObject
	deleteAllObjectVersionsFn = s3.DeleteAllObjectVersions
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

	if err := s3.DetectBucketVersioning(ctx, bucketName); err != nil {
		utils.PrintW("failed to detect bucket versioning for %s: %v\n", bucketName, err)
	} else if s3.IsBucketVersioningEnabled() {
		utils.PrintI("bucket %s is versioned\n", bucketName)
	} else {
		utils.PrintI("bucket %s is non-versioned\n", bucketName)
	}

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

	groupsToDelete := selectBackupGroupsToDelete(objects, keep)
	if len(groupsToDelete) == 0 {
		utils.PrintI("No old backups to clean up\n")
		return nil
	}

	for _, group := range groupsToDelete {
		utils.PrintI("Deleting backup: %s\n", group.prefix)
		for _, key := range group.keys {
			if err := deleteBackupObject(ctx, bucketName, key); err != nil {
				return fmt.Errorf("failed to delete %s: %w", key, err)
			}
		}
	}

	utils.PrintI("Cleanup completed\n")
	return nil
}

type backupGroup struct {
	prefix       string
	lastModified time.Time
	keys         []string
}

func selectBackupGroupsToDelete(objects []s3.ObjectInfo, keep int) []backupGroup {
	if keep <= 0 || len(objects) == 0 {
		return nil
	}

	groups := make(map[string]*backupGroup)
	for _, obj := range objects {
		prefix, ok := backupGroupPrefix(obj.Key)
		if !ok {
			continue
		}

		group := groups[prefix]
		if group == nil {
			group = &backupGroup{prefix: prefix}
			groups[prefix] = group
		}
		if obj.LastModified.After(group.lastModified) {
			group.lastModified = obj.LastModified
		}
		group.keys = append(group.keys, obj.Key)
	}

	if len(groups) <= keep {
		return nil
	}

	orderedGroups := make([]backupGroup, 0, len(groups))
	for _, group := range groups {
		sort.Strings(group.keys)
		orderedGroups = append(orderedGroups, *group)
	}

	sort.Slice(orderedGroups, func(i, j int) bool {
		if orderedGroups[i].lastModified.Equal(orderedGroups[j].lastModified) {
			return orderedGroups[i].prefix > orderedGroups[j].prefix
		}
		return orderedGroups[i].lastModified.After(orderedGroups[j].lastModified)
	})

	return orderedGroups[keep:]
}

func deleteBackupObject(ctx context.Context, bucketName, key string) error {
	if s3.IsBucketVersioningEnabled() {
		return deleteAllObjectVersionsFn(ctx, bucketName, key)
	}
	return deleteObjectFn(ctx, bucketName, key)
}

func backupGroupPrefix(key string) (string, bool) {
	parts := strings.Split(key, "/")
	if len(parts) < 5 || !strings.HasPrefix(parts[0], "backup-") {
		return "", false
	}

	return strings.Join(parts[:4], "/"), true
}
