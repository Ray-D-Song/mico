package cmd

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/require"
)

func TestCollectNetworksUsesNamesAndSkipsBuiltins(t *testing.T) {
	containers := []container.Summary{
		{
			NetworkSettings: &container.NetworkSettingsSummary{
				Networks: map[string]*network.EndpointSettings{
					"bridge":         {NetworkID: "builtin-id"},
					"tests_mico-net": {NetworkID: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
				},
			},
		},
	}

	networks := collectNetworks(containers)

	require.Equal(t, []string{"tests_mico-net"}, networks)
}

func TestIsLikelyNetworkID(t *testing.T) {
	require.True(t, isLikelyNetworkID("0123456789abcdef0123456789abcdef"))
	require.False(t, isLikelyNetworkID("tests_mico-net"))
	require.False(t, isLikelyNetworkID("bridge"))
}
