package cmd

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ray-d-song/mico/pkg/core"
	"github.com/ray-d-song/mico/pkg/docker"
	"github.com/ray-d-song/mico/pkg/runtime"
	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	verifyChecksum bool
	forceRestore   bool
)

var unpackCmd = &cobra.Command{
	Use:   "unpack",
	Short: "Unpack a migration archive and restore containers",
	Long: `Unpack command extracts a migration archive, imports images,
restores volumes, and recreates containers on the target server.

Examples:
  mico unpack migration.zst                     # Unpack with default settings
  mico unpack migration.zst --verify            # Verify checksum before unpack
  mico unpack migration.zst --force             # Force restore (overwrite existing)`,
	Run: func(cmd *cobra.Command, args []string) {
		if concurrent <= 0 {
			concurrent = 1
		}

		fmt.Print(utils.Logo)

		if len(args) == 0 {
			utils.PrintErrMsg(utils.ErrInvalidInput, "migration package required")
			return
		}

		packagePath := args[0]

		if !utils.FileExists(packagePath) {
			utils.PrintErrMsg(utils.ErrFileRead, "package not found: "+packagePath)
			return
		}

		utils.PrintI("Package: %s\n", packagePath)

		if verifyChecksum {
			utils.PrintI("Verifying checksum...\n")
			valid, err := utils.VerifyChecksum(packagePath)
			if err != nil {
				utils.PrintErrMsg(utils.ErrVerifyFailed, err)
				return
			}
			if !valid {
				utils.PrintErrMsg(utils.ErrVerifyFailed, "checksum mismatch")
				return
			}
			utils.PrintS("Checksum verified\n")
		}

		workDir := utils.CreateWorkDir("mico-unpack")
		utils.PrintI("Work directory: %s\n", workDir)

		utils.PrintI("Decompressing...\n")
		tarPath := packagePath + ".tar"
		if err := utils.NewZSTDCompressor().DecompressFile(packagePath, tarPath); err != nil {
			utils.PrintErrMsg(utils.ErrUnpackExtract, err)
			return
		}
		utils.PrintS("Decompressed\n")

		utils.PrintI("Extracting tar...\n")
		if err := extractTar(tarPath, workDir); err != nil {
			utils.PrintErrMsg(utils.ErrUnpackExtract, err)
			return
		}
		os.Remove(tarPath)
		utils.PrintS("Extracted\n")

		if !forceRestore {
			utils.PrintI("Checking port conflicts...\n")
			if err := checkPortsConflict(workDir); err != nil {
				utils.PrintErrMsg(utils.ErrVerifyFailed, err)
				return
			}
			utils.PrintS("Port check passed\n")
		}

		utils.PrintI("Loading images...\n")
		if err := loadImages(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrImagePull, err)
			return
		}
		utils.PrintS("Images loaded\n")

		utils.PrintI("Restoring volumes...\n")
		if err := restoreVolumes(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrVolumeRestore, err)
			return
		}
		utils.PrintS("Volumes restored\n")

		utils.PrintI("Creating containers...\n")
		if err := createContainers(workDir); err != nil {
			utils.PrintErrMsg(utils.ErrContainerInspect, err)
			return
		}
		utils.PrintS("Containers created\n")

		utils.PrintS("Unpack completed successfully!\n")
	},
}

func init() {
	rootCmd.AddCommand(unpackCmd)

	unpackCmd.Flags().BoolVarP(&verifyChecksum, "verify", "v", false, "Verify checksum before unpack")
	unpackCmd.Flags().BoolVarP(&forceRestore, "force", "f", false, "Force restore (overwrite existing)")
	unpackCmd.Flags().IntVarP(&concurrent, "concurrent", "j", 1, "Number of concurrent operations")
}

func extractTar(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := utils.EnsureFile(target); err != nil {
				return err
			}
			w, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			w.Close()
		}
	}
	return nil
}

func loadImages(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var loadErr error
	sem := make(chan struct{}, concurrent)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		imagePath := filepath.Join(workDir, entry.Name(), "image", "image.tar")
		if !utils.FileExists(imagePath) {
			continue
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := docker.LoadImage(path); err != nil {
				loadErr = err
			}
		}(imagePath)
	}

	wg.Wait()
	return loadErr
}

func restoreVolumes(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return fmt.Errorf("failed to read work dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerName := entry.Name()
		mountsPath := filepath.Join(workDir, containerName, "config", "mounts.json")

		if !utils.FileExists(mountsPath) {
			continue
		}

		data, err := os.ReadFile(mountsPath)
		if err != nil {
			return fmt.Errorf("failed to read mounts.json: %w", err)
		}

		var containerMounts core.ContainerMounts
		if err := json.Unmarshal(data, &containerMounts); err != nil {
			return fmt.Errorf("failed to parse mounts.json: %w", err)
		}

		for _, mnt := range containerMounts.Mounts {
			switch mnt.Type {
			case "bind":
				volumeName := generateVolumeName(containerName, mnt.Destination)
				bindTarPath := filepath.Join(workDir, containerName, "bind", escapePath(mnt.Destination)+".tar")

				if err := restoreBindVolume(volumeName, bindTarPath, forceRestore); err != nil {
					return fmt.Errorf("failed to restore volume for %s: %w", mnt.Destination, err)
				}

				utils.PrintS("Restored volume: %s\n", volumeName)

			case "volume":
				srcVolName := sourceVolumeName(mnt.Source)
				volumeName := generateVolumeName(containerName, mnt.Destination)
				volTarPath := filepath.Join(workDir, containerName, "volume", srcVolName+".tar")

				if err := restoreBindVolume(volumeName, volTarPath, forceRestore); err != nil {
					return fmt.Errorf("failed to restore volume %s: %w", srcVolName, err)
				}

				utils.PrintS("Restored volume: %s\n", volumeName)
			}
		}
	}

	return nil
}

func sourceVolumeName(source string) string {
	// source is like /var/lib/docker/volumes/<name>/_data
	parts := strings.Split(source, "/volumes/")
	if len(parts) == 2 {
		sub := strings.Split(parts[1], "/")
		if len(sub) > 0 {
			return sub[0]
		}
	}
	// fallback: take last meaningful path component
	parts = strings.Split(strings.TrimRight(source, "/"), "/")
	if len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return parts[len(parts)-1]
}

func generateVolumeName(containerName, destPath string) string {
	hash := quickHash(containerName + destPath)
	return fmt.Sprintf("mico_%s_%s", containerName, hash[:8])
}

func quickHash(s string) string {
	h := 0
	for i, c := range s {
		h = h*31 + int(c) + i
	}
	return fmt.Sprintf("%x", h)
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

func restoreBindVolume(volumeName, tarPath string, force bool) error {
	if !utils.FileExists(tarPath) {
		return nil
	}

	if force {
		exec.Command(runtime.Binary(), "volume", "rm", "-f", volumeName).Run()
	}

	cmd := exec.Command(runtime.Binary(), "volume", "create", volumeName)
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("volume create output: %s\n", string(output))
	}

	// Extract tar locally so we can copy extracted contents to the volume
	extractDir, err := os.MkdirTemp("", "mico-vol-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(extractDir)

	cmd = exec.Command("tar", "-xf", tarPath, "-C", extractDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to extract volume tar: %w, output: %s", err, string(output))
	}

	tmpContainer := "mico-restore-" + quickHash(tarPath)[:8]

	cmd = exec.Command(runtime.Binary(), "run", "-d", "--name", tmpContainer, "-v", volumeName+":/data", "alpine:latest", "sleep", "60")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create temp container: %w, output: %s", err, string(output))
	}

	cmd = exec.Command(runtime.Binary(), "cp", extractDir+"/.", tmpContainer+":/data/.")
	if output, err := cmd.CombinedOutput(); err != nil {
		dockerRemove(tmpContainer)
		return fmt.Errorf("failed to copy to volume: %w, output: %s", err, string(output))
	}

	dockerRemove(tmpContainer)
	return nil
}

func dockerRemove(name string) {
	cmd := exec.Command(runtime.Binary(), "rm", "-f", name)
	cmd.Run()
}

func checkPortsConflict(workDir string) error {
	manifestPath := filepath.Join(workDir, "manifest.json")
	if !utils.FileExists(manifestPath) {
		return nil
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest core.PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	portSet := make(map[string]bool)
	for _, svc := range manifest.Services {
		for _, port := range svc.Ports {
			port = strings.Split(port, "/")[0]
			if port != "" && port != "0" {
				portSet[port] = true
			}
		}
	}

	for port := range portSet {
		ln, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return fmt.Errorf("port %s is already in use", port)
		}
		ln.Close()
	}

	return nil
}

func restoreNetworks(workDir string) error {
	manifestPath := filepath.Join(workDir, "manifest.json")
	if !utils.FileExists(manifestPath) {
		return nil
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest core.PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	networkSet := make(map[string]bool)
	for _, networkID := range manifest.Networks {
		networkSet[networkID] = true
	}

	for networkID := range networkSet {
		cmd := exec.Command(runtime.Binary(), "network", "inspect", networkID)
		if err := cmd.Run(); err != nil {
			cmd = exec.Command(runtime.Binary(), "network", "create", networkID)
			if output, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("network create output: %s\n", string(output))
			}
		}
		utils.PrintS("Network ready: %s\n", networkID[:12])
	}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		networkPath := filepath.Join(workDir, entry.Name(), "config", "network.json")
		if !utils.FileExists(networkPath) {
			continue
		}

		data, err := os.ReadFile(networkPath)
		if err != nil {
			continue
		}

		var networkSettings struct {
			Networks map[string]interface{} `json:"Networks"`
		}
		if err := json.Unmarshal(data, &networkSettings); err != nil {
			continue
		}

		for networkName := range networkSettings.Networks {
			if isBuiltinNetwork(networkName) {
				continue
			}
			cmd := exec.Command(runtime.Binary(), "network", "inspect", networkName)
			if err := cmd.Run(); err != nil {
				cmd = exec.Command(runtime.Binary(), "network", "create", networkName)
				if output, err := cmd.CombinedOutput(); err != nil {
					fmt.Printf("network create output: %s\n", string(output))
				}
				utils.PrintS("Network created: %s\n", networkName)
			}
		}
	}

	return nil
}

func createContainers(workDir string) error {
	manifestPath := filepath.Join(workDir, "manifest.json")
	if !utils.FileExists(manifestPath) {
		return fmt.Errorf("manifest not found")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest core.PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	if err := restoreNetworks(workDir); err != nil {
		return fmt.Errorf("failed to restore networks: %w", err)
	}

	sortedServices := topologicalSortCreate(manifest.Services)
	utils.PrintS("core.Services start order: %v\n", getStartOrderNames(sortedServices))

	for _, svc := range sortedServices {
		cName := svc.ContainerName
		if cName == "" {
			cName = svc.Name
		}

		containerPath := filepath.Join(workDir, cName)
		if !utils.FileExists(containerPath) {
			continue
		}

		configPath := filepath.Join(containerPath, "config", "config.json")
		hostPath := filepath.Join(containerPath, "config", "host.json")
		if !utils.FileExists(configPath) || !utils.FileExists(hostPath) {
			continue
		}

		cfgData, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config: %w", err)
		}
		hostData, err := os.ReadFile(hostPath)
		if err != nil {
			return fmt.Errorf("failed to read host config: %w", err)
		}

		var cfg struct {
			Image        string                 `json:"Image"`
			Cmd          []string               `json:"Cmd"`
			Env          []string               `json:"Env"`
			ExposedPorts map[string]interface{} `json:"ExposedPorts"`
		}
		if err := json.Unmarshal(cfgData, &cfg); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}

		var host struct {
			NetworkMode  string   `json:"NetworkMode"`
			Binds        []string `json:"Binds"`
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		}
		if err := json.Unmarshal(hostData, &host); err != nil {
			return fmt.Errorf("failed to parse host config: %w", err)
		}

		var binds []string
		for _, bind := range host.Binds {
			parts := strings.Split(bind, ":")
			if len(parts) >= 2 {
				volumeName := generateVolumeName(cName, parts[1])
				binds = append(binds, volumeName+":"+strings.Join(parts[1:], ":"))
			} else {
				binds = append(binds, bind)
			}
		}

		var portMappings []string
		for containerPort, bindings := range host.PortBindings {
			for _, binding := range bindings {
				if binding.HostPort != "" && binding.HostPort != "0" {
					portMappings = append(portMappings, binding.HostPort+":"+containerPort)
				}
			}
		}

		runArgs := []string{"run", "-d", "--name", cName}
		for _, bind := range binds {
			runArgs = append(runArgs, "-v", bind)
		}
		for _, port := range portMappings {
			runArgs = append(runArgs, "-p", port)
		}
		for _, env := range cfg.Env {
			runArgs = append(runArgs, "-e", env)
		}
		if host.NetworkMode != "" && host.NetworkMode != "default" {
			runArgs = append(runArgs, "--net", host.NetworkMode)
		}
		if host.RestartPolicy.Name != "" && host.RestartPolicy.Name != "no" {
			runArgs = append(runArgs, "--restart", host.RestartPolicy.Name)
		}
		runArgs = append(runArgs, cfg.Image)
		if len(cfg.Cmd) > 0 {
			runArgs = append(runArgs, cfg.Cmd...)
		}

		if forceRestore {
			exec.Command(runtime.Binary(), "rm", "-f", cName).Run()
		}

		cmd := exec.Command(runtime.Binary(), runArgs...)
		if output, err := cmd.CombinedOutput(); err != nil {
			utils.PrintW("Container %s: %s\n", cName, string(output))
			continue
		}

		utils.PrintS("Container created: %s\n", cName)
	}

	return nil
}

func topologicalSortCreate(services []core.Service) []core.Service {
	return sortServicesByDeps(services)
}

func isBuiltinNetwork(name string) bool {
	switch name {
	case "bridge", "host", "none":
		return true
	}
	return false
}

func getStartOrderNames(services []core.Service) []string {
	names := make([]string, len(services))
	for i, s := range services {
		names[i] = s.Name
	}
	return names
}
