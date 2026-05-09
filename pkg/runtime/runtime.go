package runtime

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Runtime string

const (
	Docker Runtime = "docker"
	Podman Runtime = "podman"
)

// Info holds the detected container runtime configuration.
type Info struct {
	Type   Runtime // Docker or Podman
	Binary string  // Binary name for exec
	Socket string  // Socket path for API (empty means default)
}

var detected Info

// Detect checks for Podman first, falls back to Docker.
// Must be called before any container operations.
func Detect() Info {
	detected = detectRuntime()
	return detected
}

// Get returns the already-detected runtime info, or detects if not yet done.
func Get() Info {
	if detected.Type == "" {
		return Detect()
	}
	return detected
}

// Binary returns the runtime binary name (docker or podman).
func Binary() string {
	return Get().Binary
}

func detectRuntime() Info {
	// Check for rootless Podman socket
	uid := os.Getuid()
	rootlessSocket := "/run/user/" + strconv.Itoa(uid) + "/podman/podman.sock"
	if socketReachable(rootlessSocket) {
		return Info{Type: Podman, Binary: "podman", Socket: rootlessSocket}
	}

	// Check for rootful Podman socket
	rootfulSocket := "/run/podman/podman.sock"
	if socketReachable(rootfulSocket) {
		return Info{Type: Podman, Binary: "podman", Socket: rootfulSocket}
	}

	// Check if podman binary exists (might use Docker-compatible socket)
	if _, err := exec.LookPath("podman"); err == nil {
		// Podman installed but no socket - try docker.sock compatibility
		dockerSocket := "/var/run/docker.sock"
		if socketReachable(dockerSocket) {
			return Info{Type: Podman, Binary: "podman", Socket: ""}
		}
		return Info{Type: Podman, Binary: "podman", Socket: ""}
	}

	// Fall back to Docker
	return Info{Type: Docker, Binary: "docker", Socket: ""}
}

// socketReachable checks if a Unix socket exists and is accessible.
func socketReachable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

// SudoNeeded returns true if the user likely needs sudo for volume access.
func SudoNeeded() bool {
	rt := Get()
	if rt.Type == Podman && os.Geteuid() != 0 {
		// Rootless Podman stores volumes in user's home, no sudo needed
		return false
	}

	// Docker always stores volumes in /var/lib/docker (needs root)
	if rt.Type == Docker && os.Geteuid() != 0 {
		dataRoot := "/var/lib/docker/volumes"
		f, err := os.Open(dataRoot)
		if err == nil {
			f.Close()
			return false
		}
		return os.IsPermission(err)
	}

	return false
}

// DataRoot returns the container data root for permission checks.
func DataRoot() string {
	rt := Get()
	if rt.Type == Podman {
		if os.Geteuid() == 0 {
			return "/var/lib/containers/storage/volumes"
		}
		return os.Getenv("HOME") + "/.local/share/containers/storage/volumes"
	}
	return "/var/lib/docker/volumes"
}

// Version returns a human-readable runtime version string.
func Version() string {
	rt := Get()
	out, err := exec.Command(rt.Binary, "--version").Output()
	if err != nil {
		return string(rt.Type)
	}
	return strings.TrimSpace(strings.TrimPrefix(string(out), rt.Binary+" version "))
}
