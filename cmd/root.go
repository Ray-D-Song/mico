package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/ray-d-song/mico/pkg/docker"
	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/spf13/cobra"
)

func elevateIfNeeded(cmd *cobra.Command, args []string) {
	if runtime.GOOS != "linux" {
		return
	}
	if os.Geteuid() == 0 {
		return
	}

	f, err := os.Open("/var/lib/docker/volumes")
	if err == nil {
		f.Close()
		return
	}
	if !os.IsPermission(err) {
		return
	}

	fmt.Print(utils.Logo)
	utils.PrintI("Docker volume data requires root access. Elevating with sudo...\n\n")

	exe, _ := os.Executable()
	sudoArgs := append([]string{exe}, os.Args[1:]...)

	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoCmd.Stdin = os.Stdin
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr

	if err := sudoCmd.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mico",
	Short: "A simple and reliable Docker container migration tool",
	Long: `Mico is a Docker container migration tool that allows seamless migration 
of Docker container services between different servers. 

Use 'mico pack' to create a migration package containing all container services
and 'mico unpack' to restore all services on the target server.`,
	PersistentPreRun: elevateIfNeeded,
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

	// Initialize temp directory cleanup
	utils.InitTempCleanup()

	// Register cleanup on exit
	defer func() {
		utils.Cleanup()
		docker.CloseClient()
	}()

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
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
