package packer

import (
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/ray-d-song/mico/pkg/core"
	"github.com/ray-d-song/mico/pkg/deps"
	"github.com/stretchr/testify/require"
)

func TestBuildServicesFiltersContainersOutsidePack(t *testing.T) {
	depGraph := deps.DepAnalysis{
		Containers: []deps.ContainerDepInfo{
			{
				ContainerName: "mico-web",
				ServiceName:   "web",
				DependsOn:     []string{"db"},
			},
			{
				ContainerName: "mico-db",
				ServiceName:   "db",
			},
		},
	}
	containers := []container.Summary{
		{
			Names: []string{"/mico-web"},
			Image: "nginx:alpine",
			Ports: []container.Port{
				{PublicPort: 8080, Type: "tcp"},
			},
		},
	}

	services := buildServices(depGraph, containers, nil)

	require.Len(t, services, 1)
	require.Equal(t, "web", services[0].Name)
	require.Equal(t, "mico-web", services[0].ContainerName)
	require.Equal(t, "nginx:alpine", services[0].Image)
	require.Equal(t, []string{"db"}, services[0].DependsOn)
	require.Equal(t, []string{"8080/tcp"}, services[0].Ports)
	require.Equal(t, 0, services[0].StartOrder)
}

func TestBuildServicesStoresConfigHash(t *testing.T) {
	depGraph := deps.DepAnalysis{
		Containers: []deps.ContainerDepInfo{
			{
				ContainerName: "mico-web",
				ServiceName:   "web",
			},
		},
	}
	containers := []container.Summary{
		{
			Names: []string{"/mico-web"},
			Image: "nginx:alpine",
		},
	}
	cfg := &container.Config{
		Image: "nginx:alpine",
		Env:   []string{"A=B"},
	}

	services := buildServices(depGraph, containers, map[string]*container.Config{"mico-web": cfg})

	require.Len(t, services, 1)
	require.Equal(t, hashContainerConfig(cfg), services[0].ConfigHash)
}

func TestTopologicalSortIgnoresDependenciesOutsideArchive(t *testing.T) {
	services := []core.Service{
		{Name: "web", ContainerName: "mico-web", DependsOn: []string{"db"}},
	}

	sorted := core.SortServicesByDeps(services)

	require.Len(t, sorted, 1)
	require.Equal(t, "web", sorted[0].Name)
	require.Equal(t, 0, sorted[0].StartOrder)
}

func TestTopologicalSortOrdersInternalDependencies(t *testing.T) {
	services := []core.Service{
		{Name: "web", ContainerName: "mico-web", DependsOn: []string{"db"}},
		{Name: "db", ContainerName: "mico-db"},
		{Name: "worker", ContainerName: "mico-worker", DependsOn: []string{"db"}},
	}

	sorted := core.SortServicesByDeps(services)

	require.Len(t, sorted, 3)
	require.Equal(t, "db", sorted[0].Name)
	require.ElementsMatch(t, []string{"web", "worker"}, []string{sorted[1].Name, sorted[2].Name})
	for i, svc := range sorted {
		require.Equal(t, i, svc.StartOrder)
	}
}

func TestTopologicalSortKeepsServicesInCycles(t *testing.T) {
	services := []core.Service{
		{Name: "a", DependsOn: []string{"b"}},
		{Name: "b", DependsOn: []string{"a"}},
	}

	sorted := core.SortServicesByDeps(services)

	require.Len(t, sorted, 2)
	require.Equal(t, []string{"a", "b"}, []string{sorted[0].Name, sorted[1].Name})
	require.Equal(t, 0, sorted[0].StartOrder)
	require.Equal(t, 1, sorted[1].StartOrder)
}

func TestComputeDiffKeepsUnchangedComposeContainer(t *testing.T) {
	cfg := &container.Config{Image: "nginx:alpine", Env: []string{"A=B"}}
	inspectFn := func(containerName string) (*container.Config, error) {
		require.Equal(t, "mico-web", containerName)
		return cfg, nil
	}

	changed, err := computeDiff(core.PackageManifest{
		Services: []core.Service{
			{
				Name:          "web",
				ContainerName: "mico-web",
				Image:         "nginx:alpine",
				ConfigHash:    hashContainerConfig(cfg),
				Ports:         []string{"8080/tcp"},
			},
		},
	}, []container.Summary{
		{
			Names: []string{"/mico-web"},
			Image: "nginx:alpine",
			Ports: []container.Port{
				{PublicPort: 8080, Type: "tcp"},
				{PrivatePort: 80, PublicPort: 0, Type: "tcp"},
			},
		},
	}, inspectFn)

	require.NoError(t, err)
	require.Empty(t, changed)
}

func TestComputeDiffDetectsConfigChange(t *testing.T) {
	oldCfg := &container.Config{Image: "nginx:alpine", Env: []string{"A=B"}}
	newCfg := &container.Config{Image: "nginx:alpine", Env: []string{"A=C"}}
	inspectFn := func(containerName string) (*container.Config, error) {
		require.Equal(t, "mico-web", containerName)
		return newCfg, nil
	}

	changed, err := computeDiff(core.PackageManifest{
		Services: []core.Service{
			{
				Name:          "web",
				ContainerName: "mico-web",
				Image:         "nginx:alpine",
				ConfigHash:    hashContainerConfig(oldCfg),
			},
		},
	}, []container.Summary{
		{
			Names: []string{"/mico-web"},
			Image: "nginx:alpine",
		},
	}, inspectFn)

	require.NoError(t, err)
	require.Equal(t, []string{"mico-web"}, changed)
}

func TestComputeDiffSupportsOldManifestWithoutConfigHash(t *testing.T) {
	changed, err := computeDiff(core.PackageManifest{
		Services: []core.Service{
			{
				Name:          "web",
				ContainerName: "mico-web",
				Image:         "nginx:alpine",
				Ports:         []string{"8080/tcp"},
			},
		},
	}, []container.Summary{
		{
			Names: []string{"/mico-web"},
			Image: "nginx:alpine",
			Ports: []container.Port{
				{PublicPort: 8080, Type: "tcp"},
			},
		},
	}, nil)

	require.NoError(t, err)
	require.Empty(t, changed)
}
