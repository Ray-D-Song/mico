package deps

import (
	"strings"

	"github.com/docker/docker/api/types/container"
)

func AnalyzeComposeDeps(summary []container.Summary) DepGraph {
	graph := DepGraph{
		Services: make([]ServiceDep, 0, len(summary)),
	}

	for _, c := range summary {
		serviceName := c.Labels["com.docker.compose.service"]
		if serviceName == "" {
			serviceName = c.Names[0]
			if len(serviceName) > 0 && serviceName[0] == '/' {
				serviceName = serviceName[1:]
			}
		}

		project := c.Labels["com.docker.compose.project"]
		if graph.Project == "" && project != "" {
			graph.Project = project
		}

		depsStr := c.Labels["com.docker.compose.depends_on"]
		var deps []string
		if depsStr != "" {
			deps = parseDependsOn(depsStr)
		}

		dep := ServiceDep{
			ServiceName: serviceName,
			DependsOn:   deps,
		}
		graph.Services = append(graph.Services, dep)
	}

	return graph
}

func parseDependsOn(s string) []string {
	var result []string
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

