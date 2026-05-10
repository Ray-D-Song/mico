package cmd

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

	services := buildServices(depGraph, containers)

	require.Len(t, services, 1)
	require.Equal(t, "web", services[0].Name)
	require.Equal(t, "mico-web", services[0].ContainerName)
	require.Equal(t, "nginx:alpine", services[0].Image)
	require.Equal(t, []string{"db"}, services[0].DependsOn)
	require.Equal(t, []string{"8080/tcp"}, services[0].Ports)
	require.Equal(t, 0, services[0].StartOrder)
}

func TestTopologicalSortIgnoresDependenciesOutsideArchive(t *testing.T) {
	services := []core.Service{
		{Name: "web", ContainerName: "mico-web", DependsOn: []string{"db"}},
	}

	sorted := topologicalSort(services)

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

	sorted := topologicalSort(services)

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

	sorted := topologicalSort(services)

	require.Len(t, sorted, 2)
	require.Equal(t, []string{"a", "b"}, []string{sorted[0].Name, sorted[1].Name})
	require.Equal(t, 0, sorted[0].StartOrder)
	require.Equal(t, 1, sorted[1].StartOrder)
}
