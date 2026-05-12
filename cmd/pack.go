package cmd

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/ray-d-song/mico/pkg/docker"
	"github.com/ray-d-song/mico/pkg/packer"
	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	outputPath             string
	containers             string
	incremental            bool
	inspectContainerConfig = func(containerName string) (*container.Config, error) {
		client := docker.GetClient()
		resp, err := client.ContainerInspect(nil, containerName)
		if err != nil {
			return nil, err
		}
		return resp.Config, nil
	}
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

		fmt.Print(utils.Logo)

		packer.Pack(ctx, packer.PackOptions{
			OutputPath:    outputPath,
			Containers:    containers,
			Incremental:   incremental,
			Concurrent:    concurrent,
			InspectConfig: inspectContainerConfig,
		})
	},
}

func init() {
	rootCmd.AddCommand(packCmd)

	packCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for migration package (e.g., ./migration.zst)")
	packCmd.Flags().StringVarP(&containers, "containers", "c", "", "Comma-separated list of container names to pack (default: all running containers)")
	packCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations (default: 1)")
	packCmd.Flags().BoolVar(&incremental, "incremental", false, "Create incremental pack based on last pack")
}
