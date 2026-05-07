package cmd

import (
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
	"github.com/spf13/cobra"
)

var (
	outputPath   string
	containers  string
	incremental bool
)

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Pack all running containers into a migration archive",
	Long: `Pack command scans all running Docker containers, exports their images,
saves their configurations, backs up data volumes, and creates a compressed
migration package that can be transferred to another server.

Examples:
  mico pack                                    # Pack all containers with default name
  mico pack -o ./output/migration.zst         # Specify output path and filename
  mico pack -c container1,container2          # Pack specific containers only
  mico pack -c web,db -o production.zst       # Pack specific containers with custom name
  mico pack -c web,db -j 4                    # Pack with 4 concurrent workers`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		if outputPath == "" {
			timestamp := time.Now().Format("20060102150405")
			outputPath = fmt.Sprintf("mico-%s.zstd", timestamp)
		}

		utils.PrintI("Output path: %s\n", outputPath)

		scanner := docker.NewScanner()

		var containerList []container.Summary
		var err error

		if containers != "" {
			names := strings.Split(containers, ",")
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
		utils.PrintS("Project: %s\n", depGraph.Project)

		var containerNames []string
		var workDir string
		var needPackContainers []container.Summary
		var lastManifest *core.LastManifest

		if incremental {
			lastManifest, err = utils.LoadLastManifest()
			if err != nil {
				utils.PrintErrMsg(utils.ErrPackCreate, err)
				return
			}
			if lastManifest == nil {
				utils.PrintW("No previous manifest found, doing full pack\n")
				incremental = false
			} else {
				workDir = utils.CreateWorkDir("mico-incr")
				utils.PrintI("Work directory: %s\n", workDir)

				changed, err := computeDiff(lastManifest.Manifest, containerList)
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

		if !incremental {
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

		imageItems := make([]struct {
			ContainerName string
			ImageRef     string
		}, len(needPackContainers))
		for i, c := range needPackContainers {
			imageItems[i] = struct {
				ContainerName string
				ImageRef     string
			}{
				ContainerName: cleanContainerName(c.Names),
				ImageRef:     c.Image,
			}
		}

		wg.Add(3)
		go func() {
			defer wg.Done()
			saveErr = saver.SaveBatch(ctx, imageItems, concurrent)
		}()
		go func() {
			defer wg.Done()
			_, inspectErr = inspector.InspectBatch(ctx, containerNames, concurrent)
		}()
		go func() {
			defer wg.Done()
			volumeErr = volume.BackupBatch(ctx, containerNames, concurrent)
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
		if incremental {
			manifest = core.PackageManifest{
				Version:    "1.0",
				CreatedAt: time.Now(),
				Project:   depGraph.Project,
				Networks:  collectNetworks(needPackContainers),
				Services:  buildServices(depGraph, needPackContainers),
				Incremental: true,
				BasePack:     lastManifest.PackageHash,
			}
		} else {
			networks := collectNetworks(containerList)
			services := buildServices(depGraph, containerList)
			manifest = core.PackageManifest{
				Version:    "1.0",
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
		if err := os.WriteFile(manifestPath, data, 0644); err != nil {
			utils.PrintErrMsg(utils.ErrPackCreate, err)
			return
		}

		utils.PrintS("Saved images, configs, and volumes\n")

		utils.PrintI("Compressing...\n")
		compressor := utils.NewZSTDCompressor()
		if err := compressor.CompressDir(workDir, outputPath); err != nil {
			utils.PrintErrMsg(utils.ErrPackCreate, err)
			return
		}
		utils.PrintS("Compressed to: %s\n", outputPath)

		utils.PrintI("Generating checksum...\n")
		hash, err := utils.GenerateChecksumFile(outputPath)
		if err != nil {
			utils.PrintErrMsg(utils.ErrVerifyFailed, err)
			return
		}
		utils.PrintS("Checksum: %s\n", hash[:16]+"...")

		if !incremental {
			if err := utils.SaveLastManifest(hash, manifest); err != nil {
				utils.PrintErrMsg(utils.ErrPackCreate, err)
				return
			}
		}

		utils.PrintS("Pack completed successfully!\n")
	},
}

func init() {
	rootCmd.AddCommand(packCmd)

	packCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for migration package (e.g., ./migration.zst)")
	packCmd.Flags().StringVarP(&containers, "containers", "c", "", "Comma-separated list of container names to pack (default: all running containers)")
	packCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations (default: 1)")
	packCmd.Flags().BoolVar(&incremental, "incremental", false, "Create incremental pack based on last pack")
}

func collectNetworks(containers []container.Summary) []string {
	networkSet := make(map[string]bool)
	for _, c := range containers {
		for _, network := range c.NetworkSettings.Networks {
			if network != nil {
				networkSet[network.NetworkID] = true
			}
		}
	}
	networks := make([]string, 0, len(networkSet))
	for networkID := range networkSet {
		networks = append(networks, networkID)
	}
	return networks
}

func buildServices(depGraph deps.DepAnalysis, containers []container.Summary) []core.Service {
	services := make([]core.Service, 0, len(containers))
	containerMap := make(map[string]container.Summary)
	for _, c := range containers {
		name := cleanContainerName(c.Names)
		containerMap[name] = c
	}

	for _, info := range depGraph.Containers {
		var ports []string
		c, ok := containerMap[info.ContainerName]
		if ok {
			for _, port := range c.Ports {
				if port.PublicPort > 0 {
					ports = append(ports, fmt.Sprintf("%d/%s", port.PublicPort, port.Type))
				}
			}
		}

		services = append(services, core.Service{
			Name:        info.ServiceName,
			Image:      getImageFromContainer(info.ContainerName),
			DependsOn:  info.DependsOn,
			StartOrder: 0,
			Ports:     ports,
		})
	}

	sorted := topologicalSort(services)
	return sorted
}

func getImageFromContainer(containerName string) string {
	client := docker.GetClient()
	resp, err := client.ContainerInspect(nil, containerName)
	if err != nil {
		return ""
	}
	return resp.Config.Image
}

func topologicalSort(services []core.Service) []core.Service {
	if len(services) <= 1 {
		return services
	}

	inDegree := make(map[string]int)
	graph := make(map[string][]string)
	allNames := make(map[string]bool)

	for _, s := range services {
		allNames[s.Name] = true
		if _, ok := inDegree[s.Name]; !ok {
			inDegree[s.Name] = 0
		}
		for _, dep := range s.DependsOn {
			graph[dep] = append(graph[dep], s.Name)
			inDegree[s.Name]++
		}
	}

	queue := make([]string, 0)
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	result := make([]core.Service, 0, len(services))
	order := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, s := range services {
			if s.Name == current {
				s.StartOrder = order
				result = append(result, s)
				order++
				break
			}
		}

		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	return result
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

func computeDiff(lastManifest core.PackageManifest, currentContainers []container.Summary) ([]string, error) {
	lastServices := make(map[string]core.Service)
	for _, svc := range lastManifest.Services {
		lastServices[svc.Name] = svc
	}

	changed := make([]string, 0)
	for _, c := range currentContainers {
		name := cleanContainerName(c.Names)
		lastSvc, exists := lastServices[name]
		if !exists {
			changed = append(changed, name)
			continue
		}

		client := docker.GetClient()
		resp, err := client.ContainerInspect(nil, name)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
		}

		configHash := utils.SHA256String(fmt.Sprintf("%v", resp.Config))

		hasChange := false
		if lastSvc.Image != c.Image {
			hasChange = true
		}
		if !hasChange && configHash != utils.SHA256String(fmt.Sprintf("%v", lastSvc)) {
			hasChange = true
		}
		if !hasChange {
			for _, port := range c.Ports {
				found := false
				for _, p := range lastSvc.Ports {
					if p == fmt.Sprintf("%d/%s", port.PublicPort, port.Type) {
						found = true
						break
					}
				}
				if !found {
					hasChange = true
					break
				}
			}
		}

		if hasChange {
			changed = append(changed, name)
		}
	}

	return changed, nil
}
