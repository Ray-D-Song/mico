package cmd

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ray-d-song/mico/pkg/core"
	"github.com/ray-d-song/mico/pkg/docker"
	"github.com/ray-d-song/mico/pkg/runtime"
	"github.com/ray-d-song/mico/pkg/s3"
	"github.com/ray-d-song/mico/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	skipVerify bool
	forceRestore   bool
	s3Mode         bool
	s3Bucket       string
	s3Key          string
	s3List         bool
	loadImage      = docker.LoadImage
	runContainer   = func(args ...string) ([]byte, error) {
		return exec.Command(runtime.Binary(), args...).CombinedOutput()
	}
)

var unpackCmd = &cobra.Command{
	Use:   "unpack",
	Short: "Unpack a migration archive and restore containers",
	Long: `Unpack command extracts a migration archive, imports images,
restores volumes, and recreates containers on the target server.

Examples:
  mico unpack migration.zst                     # Unpack from local file (verify by default)
  mico unpack migration.zst --no-verify          # Unpack from local file without checksum verification
  mico unpack migration.zst --force              # Force restore (overwrite existing)
  mico unpack --s3 --list                       # List all backups in S3
  mico unpack --s3                              # Restore latest backup from S3
  mico unpack --s3 --key backup-2026/...        # Restore specific backup from S3`,
	Run: func(cmd *cobra.Command, args []string) {
		if concurrent <= 0 {
			concurrent = 1
		}

		fmt.Print(utils.Logo)

		ctx := cmd.Context()
		var packagePath string

		if s3Mode {
			if err := s3.InitializeClient(); err != nil {
				utils.PrintE("%s\n", err.Error())
				return
			}
			bucketName := s3Bucket
			if bucketName == "" {
				if cfg := s3.GetConfig(); cfg.Bucket != "" {
					bucketName = cfg.Bucket
				}
			}
			if bucketName == "" {
				utils.PrintE("no s3 bucket specified\n")
				return
			}

			if s3List {
				if err := listBackups(ctx, bucketName); err != nil {
					utils.PrintE("%s\n", err.Error())
				}
				return
			}

			key := s3Key
			if key == "" {
				latest, err := findLatestBackup(ctx, bucketName)
				if err != nil {
					utils.PrintE("%s\n", err.Error())
					return
				}
				if latest == "" {
					utils.PrintW("No backups found in bucket %s\n", bucketName)
					return
				}
				key = latest
				utils.PrintI("Latest backup: %s\n", key)
			}

			tmpDir := os.TempDir()
			timestamp := time.Now().Format("20060102150405")
			tmpFile := filepath.Join(tmpDir, "mico-restore-"+timestamp+".zst")
			tmpChecksum := tmpFile + ".sha256"

			checksumKey := key + ".sha256"
			utils.PrintI("Downloading checksum...\n")
			if err := s3.DownloadFile(ctx, bucketName, checksumKey, tmpChecksum); err != nil {
				utils.PrintE("checksum not found: %v\n", err.Error())
				return
			}
			defer os.Remove(tmpChecksum)

			utils.PrintI("Downloading backup...\n")
			if err := s3.DownloadFile(ctx, bucketName, key, tmpFile); err != nil {
				utils.PrintE("download failed: %v\n", err.Error())
				return
			}
			defer os.Remove(tmpFile)

			packagePath = tmpFile
		} else {
			if len(args) == 0 {
				utils.PrintErrMsg(utils.ErrInvalidInput, "migration package or --s3 flag required")
				return
			}
			packagePath = args[0]
		}

		if !utils.FileExists(packagePath) {
			utils.PrintErrMsg(utils.ErrFileRead, "package not found: "+packagePath)
			return
		}

		utils.PrintI("Package: %s\n", packagePath)

		if !skipVerify {
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

	unpackCmd.Flags().BoolVar(&s3Mode, "s3", false, "Restore backup from S3 instead of local file")
	unpackCmd.Flags().StringVarP(&s3Bucket, "bucket", "b", "", "S3 bucket name (default: from ~/.mico/s3.ini)")
	unpackCmd.Flags().StringVarP(&s3Key, "key", "k", "", "Specific backup key to restore (default: latest)")
	unpackCmd.Flags().BoolVarP(&s3List, "list", "l", false, "List available backups in S3")
	unpackCmd.Flags().BoolVar(&skipVerify, "no-verify", false, "Skip checksum verification")
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

		target, err := safeTarTarget(destDir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := utils.EnsureFile(target); err != nil {
				return err
			}
			w, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return err
			}
			if err := w.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("refusing to extract link %q", header.Name)
		}
	}
	return nil
}

func safeTarTarget(destDir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("invalid empty tar entry name")
	}

	cleanName := filepath.Clean(name)
	if filepath.IsAbs(cleanName) || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("tar entry %q escapes destination", name)
	}

	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(destAbs, cleanName))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(destAbs, targetAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("tar entry %q escapes destination", name)
	}

	return targetAbs, nil
}

func loadImages(workDir string) error {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrent)
	results := make(chan error, len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		imagePath := utils.ServiceImageTar(workDir, entry.Name())
		if !utils.FileExists(imagePath) {
			continue
		}

		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results <- loadImage(path)
		}(imagePath)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	for err := range results {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
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
		mountsPath := utils.ServiceMountsJSON(workDir, containerName)

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
				bindTarPath := utils.ServiceBindTar(workDir, containerName, mnt.Destination)

				if err := restoreBindVolume(volumeName, bindTarPath, forceRestore); err != nil {
					return fmt.Errorf("failed to restore volume for %s: %w", mnt.Destination, err)
				}

				utils.PrintS("Restored volume: %s\n", volumeName)

			case "volume":
				srcVolName := sourceVolumeName(mnt.Source)
				volumeName := generateVolumeName(containerName, mnt.Destination)
				volTarPath := utils.ServiceVolumeTar(workDir, containerName, srcVolName)

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
	destPath = strings.TrimRight(destPath, "/")
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
	manifestPath := utils.ManifestPath(workDir)
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
	manifestPath := utils.ManifestPath(workDir)
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
	for _, networkName := range manifest.Networks {
		if networkName == "" || core.IsBuiltinNetwork(networkName) || core.IsLikelyNetworkID(networkName) {
			continue
		}
		networkSet[networkName] = true
	}

	ensured := make(map[string]bool)
	ensureOnce := func(networkName string) error {
		if ensured[networkName] {
			return nil
		}
		ensured[networkName] = true
		return ensureNetwork(networkName)
	}

	for networkName := range networkSet {
		if err := ensureOnce(networkName); err != nil {
			return err
		}
	}

	entries, err := os.ReadDir(workDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		networkPath := utils.ServiceNetworkJSON(workDir, entry.Name())
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
			if core.IsBuiltinNetwork(networkName) {
				continue
			}
			if err := ensureOnce(networkName); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureNetwork(networkName string) error {
	cmd := exec.Command(runtime.Binary(), "network", "inspect", networkName)
	if err := cmd.Run(); err == nil {
		utils.PrintS("Network ready: %s\n", networkName)
		return nil
	}

	cmd = exec.Command(runtime.Binary(), "network", "create", networkName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create network %s: %w, output: %s", networkName, err, string(output))
	}
	utils.PrintS("Network created: %s\n", networkName)
	return nil
}

func createContainers(workDir string) error {
	manifestPath := utils.ManifestPath(workDir)
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

	sortedServices := core.SortServicesByDeps(manifest.Services)

	var createErrs []string
	for _, svc := range sortedServices {
		cName := svc.ContainerName
		if cName == "" {
			cName = svc.Name
		}

		containerPath := utils.ServiceDir(workDir, cName)
		if !utils.FileExists(containerPath) {
			continue
		}

		configPath := utils.ServiceConfigJSON(workDir, cName)
		hostPath := utils.ServiceHostJSON(workDir, cName)
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
				dest := strings.TrimRight(parts[1], "/")
				volumeName := generateVolumeName(cName, dest)
				binds = append(binds, volumeName+":"+dest)
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

		// Determine the correct network to start on.
		// When the original container was on a user-defined network alongside the
		// default bridge (NetworkMode == "bridge"), we must start the new container
		// directly on that user-defined network rather than connecting after start.
		effectiveNetworkMode := host.NetworkMode
		additionalNetworks := make([]string, 0)
		if networkData, err := os.ReadFile(utils.ServiceNetworkJSON(workDir, cName)); err == nil {
			var netSettings struct {
				Networks map[string]interface{} `json:"Networks"`
			}
			if err := json.Unmarshal(networkData, &netSettings); err == nil {
				utils.PrintI("[DEBUG] %s network.json networks: %v\n", cName, mapKeys(netSettings.Networks))
				utils.PrintI("[DEBUG] %s host.NetworkMode: %q\n", cName, host.NetworkMode)
				for networkName := range netSettings.Networks {
					if networkName == "" || core.IsBuiltinNetwork(networkName) || core.IsLikelyNetworkID(networkName) {
						utils.PrintI("[DEBUG] %s skipping builtin/ID network: %s\n", cName, networkName)
						continue
					}
					if effectiveNetworkMode == "" || effectiveNetworkMode == "default" || core.IsBuiltinNetwork(effectiveNetworkMode) {
						effectiveNetworkMode = networkName
						utils.PrintI("[DEBUG] %s using first custom network as primary: %s\n", cName, networkName)
					} else if networkName != effectiveNetworkMode {
						additionalNetworks = append(additionalNetworks, networkName)
					}
				}
			}
		} else {
			utils.PrintI("[DEBUG] %s no network.json found: %v\n", cName, err)
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
		if effectiveNetworkMode != "" && effectiveNetworkMode != "default" && !core.IsBuiltinNetwork(effectiveNetworkMode) {
			runArgs = append(runArgs, "--net", effectiveNetworkMode)
			utils.PrintI("[DEBUG] %s using --net %s\n", cName, effectiveNetworkMode)
		} else {
			utils.PrintI("[DEBUG] %s no --net flag (NetworkMode=%q effective=%q)\n", cName, host.NetworkMode, effectiveNetworkMode)
		}
		if host.RestartPolicy.Name != "" && host.RestartPolicy.Name != "no" {
			runArgs = append(runArgs, "--restart", host.RestartPolicy.Name)
		}
		runArgs = append(runArgs, cfg.Image)
		if len(cfg.Cmd) > 0 {
			runArgs = append(runArgs, cfg.Cmd...)
		}

		utils.PrintI("[DEBUG] %s run args: %v\n", cName, runArgs)

		if forceRestore {
			exec.Command(runtime.Binary(), "rm", "-f", cName).Run()
		}

		if output, err := runContainer(runArgs...); err != nil {
			errMsg := fmt.Sprintf("container %s: %s", cName, strings.TrimSpace(string(output)))
			utils.PrintW("%s\n", errMsg)
			createErrs = append(createErrs, errMsg)
			continue
		}

		for _, networkName := range additionalNetworks {
			utils.PrintI("[DEBUG] %s connecting to additional network: %s\n", cName, networkName)
			if output, err := runContainer("network", "connect", networkName, cName); err != nil {
				utils.PrintW("Failed to connect %s to network %s: %v output=%s\n", cName, networkName, err, strings.TrimSpace(string(output)))
			} else {
				utils.PrintS("Connected %s to network %s\n", cName, networkName)
			}
		}

		utils.PrintS("Container created: %s\n", cName)
	}

	if len(createErrs) > 0 {
		return fmt.Errorf("failed to create %d container(s): %s", len(createErrs), strings.Join(createErrs, "; "))
	}

	return nil
}

func listBackups(ctx context.Context, bucketName string) error {
	objects, err := s3.ListObjects(ctx, bucketName, "backup-", 1000)
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}
	if len(objects) == 0 {
		utils.PrintW("No backups found in bucket %s\n", bucketName)
		return nil
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})

	fmt.Printf("%-4s %-20s %-8s %s\n", "No.", "Date", "Size", "Key")
	fmt.Println(strings.Repeat("-", 80))
	for i, obj := range objects {
		if !strings.HasSuffix(obj.Key, ".sha256") {
			fmt.Printf("%-4d %-20s %8s %s\n", i+1, obj.LastModified.Format("2006-01-02 15:04:05"), formatSize(obj.Size), obj.Key)
		}
	}
	return nil
}

func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%dB", n)
}

func findLatestBackup(ctx context.Context, bucketName string) (string, error) {
	objects, err := s3.ListObjects(ctx, bucketName, "backup-", 1000)
	if err != nil {
		return "", fmt.Errorf("failed to list backups: %w", err)
	}

	var latest string
	var latestTime time.Time
	for _, obj := range objects {
		if strings.HasSuffix(obj.Key, ".sha256") {
			continue
		}
		if obj.LastModified.After(latestTime) {
			latestTime = obj.LastModified
			latest = obj.Key
		}
	}
	return latest, nil
}
