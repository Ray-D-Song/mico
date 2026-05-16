package cmd

import (
	"testing"
	"time"

	"github.com/ray-d-song/mico/pkg/s3"
	"github.com/stretchr/testify/require"
)

func TestSelectBackupGroupsToDeleteKeepsWholeBackupGroups(t *testing.T) {
	base := time.Date(2026, 5, 16, 15, 0, 0, 0, time.UTC)
	objects := []s3.ObjectInfo{
		{
			Key:          "backup-2026/05/16/150000/mico-backup.zst",
			LastModified: base,
		},
		{
			Key:          "backup-2026/05/16/150000/mico-backup.zst.sha256",
			LastModified: base.Add(1 * time.Second),
		},
		{
			Key:          "backup-2026/05/16/160000/mico-backup.zst",
			LastModified: base.Add(1 * time.Hour),
		},
		{
			Key:          "backup-2026/05/16/160000/mico-backup.zst.sha256",
			LastModified: base.Add(1*time.Hour + 1*time.Second),
		},
		{
			Key:          "backup-2026/05/16/170000/mico-backup.zst",
			LastModified: base.Add(2 * time.Hour),
		},
		{
			Key:          "backup-2026/05/16/170000/mico-backup.zst.sha256",
			LastModified: base.Add(2*time.Hour + 1*time.Second),
		},
		{
			Key:          "backup-2026/05/16/180000/mico-backup.zst",
			LastModified: base.Add(3 * time.Hour),
		},
		{
			Key:          "backup-2026/05/16/180000/mico-backup.zst.sha256",
			LastModified: base.Add(3*time.Hour + 1*time.Second),
		},
	}

	groupsToDelete := selectBackupGroupsToDelete(objects, 3)

	require.Len(t, groupsToDelete, 1)
	require.Equal(t, "backup-2026/05/16/150000", groupsToDelete[0].prefix)
	require.Equal(t, []string{
		"backup-2026/05/16/150000/mico-backup.zst",
		"backup-2026/05/16/150000/mico-backup.zst.sha256",
	}, groupsToDelete[0].keys)
}

func TestSelectBackupGroupsToDeleteIgnoresNonBackupKeys(t *testing.T) {
	base := time.Date(2026, 5, 16, 15, 0, 0, 0, time.UTC)
	objects := []s3.ObjectInfo{
		{
			Key:          "backup-2026/05/16/150000/mico-backup.zst",
			LastModified: base,
		},
		{
			Key:          "backup-2026/05/16/150000/mico-backup.zst.sha256",
			LastModified: base.Add(1 * time.Second),
		},
		{
			Key:          "misc/readme.txt",
			LastModified: base.Add(2 * time.Hour),
		},
	}

	groupsToDelete := selectBackupGroupsToDelete(objects, 1)

	require.Empty(t, groupsToDelete)
}

func TestBackupGroupPrefix(t *testing.T) {
	prefix, ok := backupGroupPrefix("backup-2026/05/16/152302/mico-backup.zst.sha256")
	require.True(t, ok)
	require.Equal(t, "backup-2026/05/16/152302", prefix)

	_, ok = backupGroupPrefix("backup-/broken")
	require.False(t, ok)

	_, ok = backupGroupPrefix("notes/backup-2026.txt")
	require.False(t, ok)
}
