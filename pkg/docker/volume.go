package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/ray-d-song/migo/pkg/utils"
)

type VolumeBackup struct {
	workDir string
}

func NewVolumeBackup(workDir string) *VolumeBackup {
	return &VolumeBackup{workDir: workDir}
}

func (v *VolumeBackup) BackupOne(ctx context.Context, containerName string) error {
	client := GetClient()
	resp, err := client.ContainerInspect(ctx, containerName)
	if err != nil {
		return fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}

	if len(resp.Mounts) == 0 {
		return nil
	}

	servicePath := filepath.Join(v.workDir, containerName)
	utils.EnsureDir(servicePath + "/volume")
	utils.EnsureDir(servicePath + "/bind")

	for _, mnt := range resp.Mounts {
		switch mnt.Type {
		case "volume":
			volumePath := filepath.Join(servicePath, "volume", mnt.Name+".tar")
			if err := v.backupVolume(ctx, mnt.Source, volumePath); err != nil {
				return fmt.Errorf("failed to backup volume %s: %w", mnt.Name, err)
			}
		case "bind":
			bindPath := filepath.Join(servicePath, "bind", escapePath(mnt.Destination)+".tar")
			if err := v.backupBind(ctx, mnt.Source, bindPath); err != nil {
				return fmt.Errorf("failed to backup bind %s: %w", mnt.Source, err)
			}
		}
	}

	return nil
}

func escapePath(path string) string {
	result := ""
	for _, c := range path {
		if c == '/' || c == ':' {
			result += "_"
		} else {
			result += string(c)
		}
	}
	return result
}

func (v *VolumeBackup) backupVolume(ctx context.Context, sourcePath, destPath string) error {
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create volume file: %w", err)
	}
	defer f.Close()

	_, err = f.Write([]byte{})
	if err != nil {
		return fmt.Errorf("failed to write volume file: %w", err)
	}

	return nil
}

func (v *VolumeBackup) backupBind(ctx context.Context, sourcePath, destPath string) error {
	info, err := os.Stat(sourcePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat bind path: %w", err)
	}
	if !info.IsDir() {
		return nil
	}

	if err := utils.EnsureFile(destPath); err != nil {
		return fmt.Errorf("failed to create bind backup dir: %w", err)
	}

	cmd := exec.Command("tar", "-cf", destPath, "-C", sourcePath, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar failed: %w, output: %s", err, string(output))
	}
	return nil
}

func (v *VolumeBackup) BackupBatch(ctx context.Context, names []string, concurrent int) error {
	if len(names) == 0 {
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
	results := make(chan result, len(names))
	var wg sync.WaitGroup

	for _, name := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			err := v.BackupOne(ctx, n)
			results <- result{containerName: n, err: err}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			return fmt.Errorf("failed to backup volume for %s: %w", r.containerName, r.err)
		}
	}

	return nil
}

func GetContainerVolumes(containerName string) ([]container.MountPoint, error) {
	client := GetClient()
	resp, err := client.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", containerName, err)
	}
	return resp.Mounts, nil
}