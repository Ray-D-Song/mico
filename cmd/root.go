package cmd

import (
	"fmt"
	"os"

	"github.com/ray-d-song/migo/pkg/docker"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mico",
	Short: "A simple and reliable Docker container migration tool",
	Long: `Mico is a Docker container migration tool that allows seamless migration 
of Docker container services between different servers. 

Use 'mico pack' to create a migration package containing all container services
and 'mico unpack' to restore all services on the target server.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Initialize Docker client at startup
	if err := docker.InitializeClient(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize Docker client: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure Docker is running and accessible.\n")
		os.Exit(1)
	}

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	
	// Clean up Docker client on exit
	docker.CloseClient()
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mico.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
