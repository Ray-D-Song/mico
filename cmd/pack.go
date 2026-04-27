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
	"github.com/ray-d-song/migo/pkg/deps"
	"github.com/ray-d-song/migo/pkg/docker"
	"github.com/ray-d-song/migo/pkg/utils"
	"github.com/spf13/cobra"
)

type Service struct {
	Name        string   `json:"name"`
	Image      string   `json:"image"`
	DependsOn  []string `json:"depends_on"`
	StartOrder int      `json:"start_order"`
	Ports      []string `json:"ports"`
}

type PackageManifest struct {
	Version    string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
	Networks  []string  `json:"networks"`
	Services  []Service `json:"services"`
}

var (
	outputPath  string
	containers string
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

		workDir := utils.CreateWorkDir("mico")
		utils.PrintI("Work directory: %s\n", workDir)

		saver := docker.NewImageSaver(workDir)
		inspector := docker.NewInspector(workDir)
		volume := docker.NewVolumeBackup(workDir)

		utils.PrintI("Saving images, configs, and volumes...\n")

		var wg sync.WaitGroup
		var saveErr, inspectErr, volumeErr error

		imageItems := make([]struct {
			ContainerName string
			ImageRef     string
		}, len(containerList))
		for i, c := range containerList {
			name := c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			imageItems[i] = struct {
				ContainerName string
				ImageRef     string
			}{
				ContainerName: name,
				ImageRef:     c.Image,
			}
		}

		containerNames := make([]string, len(containerList))
		for i, c := range containerList {
			name := c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			containerNames[i] = name
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

		networks := collectNetworks(containerList)
		services := buildServices(depGraph, containerList)
		manifest := PackageManifest{
			Version:    "1.0",
			CreatedAt: time.Now(),
			Project:   depGraph.Project,
			Networks:  networks,
			Services:  services,
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
		utils.PrintS("Pack completed successfully!\n")
	},
}

func init() {
	rootCmd.AddCommand(packCmd)

	packCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for migration package (e.g., ./migration.zst)")
	packCmd.Flags().StringVarP(&containers, "containers", "c", "", "Comma-separated list of container names to pack (default: all running containers)")
	packCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations (default: 1)")
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

func buildServices(depGraph deps.DepAnalysis, containers []container.Summary) []Service {
	services := make([]Service, 0, len(containers))
	containerMap := make(map[string]container.Summary)
	for _, c := range containers {
		name := c.Names[0]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
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

		services = append(services, Service{
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

func topologicalSort(services []Service) []Service {
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

	result := make([]Service, 0, len(services))
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
