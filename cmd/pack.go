package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	outputPath string
	containers string
)

// packCmd represents the pack command
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
  mico pack -c web,db -o production.zst       # Pack specific containers with custom name`,
	Run: func(cmd *cobra.Command, args []string) {
		// Print user input for debugging purposes

		if outputPath != "" {
			fmt.Printf("Output path: %s\n", outputPath)
		} else {
			fmt.Println("Output path: (default)")
		}

		if containers != "" {
			containerList := strings.Split(containers, ",")
			fmt.Printf("Containers to pack: %v\n", containerList)
		} else {
			fmt.Println("Containers to pack: (all running containers)")
		}

	},
}

func init() {
	// Add pack command to root command
	rootCmd.AddCommand(packCmd)

	// Define flags for pack command
	packCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path for migration package (e.g., ./migration.zst)")
	packCmd.Flags().StringVarP(&containers, "containers", "c", "", "Comma-separated list of container names to pack (default: all running containers)")
}
