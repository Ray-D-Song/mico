package core

import (
	"github.com/docker/docker/api/types/container"
)

func IsBuiltinNetwork(name string) bool {
	switch name {
	case "bridge", "host", "none":
		return true
	}
	return false
}

func CollectNetworks(containers []container.Summary) []string {
	networkSet := make(map[string]bool)
	for _, c := range containers {
		if c.NetworkSettings == nil {
			continue
		}
		for networkName := range c.NetworkSettings.Networks {
			if IsBuiltinNetwork(networkName) {
				continue
			}
			networkSet[networkName] = true
		}
	}
	networks := make([]string, 0, len(networkSet))
	for networkName := range networkSet {
		networks = append(networks, networkName)
	}
	return networks
}

func IsLikelyNetworkID(name string) bool {
	if len(name) < 32 {
		return false
	}
	for _, c := range name {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return false
	}
	return true
}
