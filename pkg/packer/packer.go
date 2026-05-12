package packer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/ray-d-song/mico/pkg/core"
	"github.com/ray-d-song/mico/pkg/deps"
	"github.com/ray-d-song/mico/pkg/docker"
	"github.com/ray-d-song/mico/pkg/utils"
)

type InspectConfig = func(containerName string) (*container.Config, error)

type PackOptions struct {
	OutputPath  string
	Containers  string
	Incremental bool
	Concurrent  int
}

func Pack(ctx context.Context, opts PackOptions) {
	if opts.OutputPath == "" {
		timestamp := time.Now().Format("20060102150405")
		opts.OutputPath = fmt.Sprintf("mico-%s.zstd", timestamp)
	}

	utils.PrintI("Output path: %s\n", opts.OutputPath)

	scanner := docker.NewScanner()

	var containerList []container.Summary
	var err error

	if opts.Containers != "" {
		names := strings.Split(opts.Containers, ",")
		utils.PrintI("Containers to pack: %v\n", names)
		containerList, err = scanner.FilterContainersByNames(ctx, names)
		if err != nil {
			utils.PrintErrMsg(utils.ErrContainerScan, err)
			return
		}
	} else {
		utils.PrintI("Containers to pack: (all running containers)\n")
		containerList, err = scanner.ScanRunningContainers(ctx)
		if err != nil {
			utils.PrintErrMsg(utils.ErrContainerScan, err)
			return
		}
	}

	utils.PrintI("Found %d running containers\n", len(containerList))

	if len(containerList) == 0 {
		utils.PrintW("No containers to pack\n")
		return
	}

	utils.PrintI("Analyzing dependencies...\n")
	depGraph := deps.AnalyzeComposeDeps(containerList)
	if depGraph.Project != "" {
		utils.PrintS("Project: %s\n", depGraph.Project)
	}

	var containerNames []string
	var workDir string
	var needPackContainers []container.Summary
	var lastManifest *core.LastManifest

	if opts.Incremental {
		lastManifest, err = utils.LoadLastManifest()
		if err != nil {
			utils.PrintErrMsg(utils.ErrPackCreate, err)
			return
		}
		if lastManifest == nil {
			utils.PrintW("No previous manifest found, doing full pack\n")
			opts.Incremental = false
		} else {
			workDir = utils.CreateWorkDir("mico-incr")
			utils.PrintI("Work directory: %s\n", workDir)

			changed, err := computeDiff(lastManifest.Manifest, containerList, inspectContainerConfig)
			if err != nil {
				utils.PrintErrMsg(utils.ErrPackCreate, err)
				return
			}

			if len(changed) == 0 {
				utils.PrintS("No changes detected, nothing to pack\n")
				return
			}

			utils.PrintI("Changed containers: %v\n", changed)
			containerNames = changed
			for _, name := range changed {
				for _, c := range containerList {
					cName := cleanContainerName(c.Names)
					if cName == name {
						needPackContainers = append(needPackContainers, c)
						break
					}
				}
			}
		}
	}

	if !opts.Incremental {
		workDir = utils.CreateWorkDir("mico")
		utils.PrintI("Work directory: %s\n", workDir)
		needPackContainers = containerList
		containerNames = make([]string, len(containerList))
		for i, c := range containerList {
			containerNames[i] = cleanContainerName(c.Names)
		}
	}

	saver := docker.NewImageSaver(workDir)
	inspector := docker.NewInspector(workDir)
	volume := docker.NewVolumeBackup(workDir)

	utils.PrintI("Saving images, configs, and volumes...\n")

	var wg sync.WaitGroup
	var saveErr, inspectErr, volumeErr error
	var configs map[string]*container.Config

	imageItems := make([]struct {
		ContainerName string
		ImageRef      string
	}, len(needPackContainers))
	for i, c := range needPackContainers {
		imageItems[i] = struct {
			ContainerName string
			ImageRef      string
		}{
			ContainerName: cleanContainerName(c.Names),
			ImageRef:      c.Image,
		}
	}

	wg.Add(3)
	go func() {
		defer wg.Done()
		saveErr = saver.SaveBatch(ctx, imageItems, opts.Concurrent)
	}()
	go func() {
		defer wg.Done()
		configs, inspectErr = inspector.InspectBatch(ctx, containerNames, opts.Concurrent)
	}()
	go func() {
		defer wg.Done()
		volumeErr = volume.BackupBatch(ctx, containerNames, opts.Concurrent)
	}()

	wg.Wait()

	if saveErr != nil {
		utils.PrintErrMsg(utils.ErrImageSave, saveErr)
		return
	}
	if inspectErr != nil {
		utils.PrintErrMsg(utils.ErrContainerInspect, inspectErr)
		return
	}
	if volumeErr != nil {
		utils.PrintErrMsg(utils.ErrVolumeBackup, volumeErr)
		return
	}

	utils.PrintI("Saving mount information...\n")
	for _, name := range containerNames {
		if _, err := inspector.SaveMounts(ctx, name); err != nil {
			utils.PrintErrMsg(utils.ErrContainerInspect, err)
			return
		}
		if err := inspector.SaveHostConfig(ctx, name); err != nil {
			utils.PrintErrMsg(utils.ErrContainerInspect, err)
			return
		}
		if err := inspector.SaveNetworkSettings(ctx, name); err != nil {
			utils.PrintErrMsg(utils.ErrContainerInspect, err)
			return
		}
	}

	var manifest core.PackageManifest
	if opts.Incremental {
		manifest = core.PackageManifest{
			Version:     "1.0",
			CreatedAt:   time.Now(),
			Project:     depGraph.Project,
			Networks:    core.CollectNetworks(needPackContainers),
			Services:    buildServices(depGraph, needPackContainers, configs),
			Incremental: true,
			BasePack:    lastManifest.PackageHash,
		}
	} else {
		networks := core.CollectNetworks(containerList)
		services := buildServices(depGraph, containerList, configs)
		manifest = core.PackageManifest{
			Version:   "1.0",
			CreatedAt: time.Now(),
			Project:   depGraph.Project,
			Networks:  networks,
			Services:  services,
		}
	}

	manifestPath := filepath.Join(workDir, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		utils.PrintErrMsg(utils.ErrPackCreate, err)
		return
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		utils.PrintErrMsg(utils.ErrPackCreate, err)
		return
	}

	utils.PrintS("Saved images, configs, and volumes\n")

	utils.PrintI("Compressing...\n")
	compressor := utils.NewZSTDCompressor()
	if err := compressor.CompressDir(workDir, opts.OutputPath); err != nil {
		utils.PrintErrMsg(utils.ErrPackCreate, err)
		return
	}
	utils.PrintS("Compressed to: %s\n", opts.OutputPath)

	utils.PrintI("Generating checksum...\n")
	hash, err := utils.GenerateChecksumFile(opts.OutputPath)
	if err != nil {
		utils.PrintErrMsg(utils.ErrVerifyFailed, err)
		return
	}
	utils.PrintS("Checksum: %s\n", hash[:16]+"...")

	if !opts.Incremental {
		if err := utils.SaveLastManifest(hash, manifest); err != nil {
			utils.PrintErrMsg(utils.ErrPackCreate, err)
			return
		}
	}

	utils.PrintS("Pack completed successfully!\n")
}

func buildServices(depGraph deps.DepAnalysis, containers []container.Summary, configs map[string]*container.Config) []core.Service {
	services := make([]core.Service, 0, len(containers))
	containerMap := make(map[string]container.Summary)
	for _, c := range containers {
		name := cleanContainerName(c.Names)
		containerMap[name] = c
	}

	for _, info := range depGraph.Containers {
		var ports []string
		c, ok := containerMap[info.ContainerName]
		if !ok {
			continue
		}

		for _, port := range c.Ports {
			if port.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d/%s", port.PublicPort, port.Type))
			}
		}

		services = append(services, core.Service{
			Name:          info.ServiceName,
			ContainerName: info.ContainerName,
			Image:         c.Image,
			ConfigHash:    hashContainerConfig(configs[info.ContainerName]),
			DependsOn:     info.DependsOn,
			StartOrder:    0,
			Ports:         ports,
		})
	}

	sorted := core.SortServicesByDeps(services)
	return sorted
}

func cleanContainerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	name := names[0]
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}

func computeDiff(lastManifest core.PackageManifest, currentContainers []container.Summary, inspectContainerConfig InspectConfig) ([]string, error) {
	lastServices := make(map[string]core.Service)
	for _, svc := range lastManifest.Services {
		if svc.ContainerName != "" {
			lastServices[svc.ContainerName] = svc
		}
		if svc.Name != "" {
			lastServices[svc.Name] = svc
		}
	}

	changed := make([]string, 0)
	for _, c := range currentContainers {
		name := cleanContainerName(c.Names)
		lastSvc, exists := lastServices[name]
		if !exists {
			changed = append(changed, name)
			continue
		}

		hasChange := false
		if lastSvc.Image != c.Image {
			hasChange = true
		}
		if !hasChange && lastSvc.ConfigHash != "" {
			cfg, err := inspectContainerConfig(name)
			if err != nil {
				return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
			}
			if hashContainerConfig(cfg) != lastSvc.ConfigHash {
				hasChange = true
			}
		}
		if !hasChange && !sameStringSet(containerPublicPorts(c), lastSvc.Ports) {
			hasChange = true
		}

		if hasChange {
			changed = append(changed, name)
		}
	}

	return changed, nil
}

func hashContainerConfig(cfg *container.Config) string {
	if cfg == nil {
		return ""
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return utils.SHA256String(fmt.Sprintf("%v", cfg))
	}
	return utils.SHA256String(string(data))
}

func containerPublicPorts(c container.Summary) []string {
	ports := make([]string, 0, len(c.Ports))
	for _, port := range c.Ports {
		if port.PublicPort > 0 {
			ports = append(ports, fmt.Sprintf("%d/%s", port.PublicPort, port.Type))
		}
	}
	return ports
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, v := range a {
		counts[v]++
	}
	for _, v := range b {
		if counts[v] == 0 {
			return false
		}
		counts[v]--
		if counts[v] == 0 {
			delete(counts, v)
		}
	}
	return len(counts) == 0
}

var inspectContainerConfig = func(containerName string) (*container.Config, error) {
	client := docker.GetClient()
	resp, err := client.ContainerInspect(nil, containerName)
	if err != nil {
		return nil, err
	}
	return resp.Config, nil
}
