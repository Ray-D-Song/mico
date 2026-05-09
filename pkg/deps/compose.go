package deps

import (
	"strings"

	"github.com/docker/docker/api/types/container"
)

// AnalyzeComposeDeps analyzes compose dependencies from container summaries
// Extracts project name, service names, and depends_on relationships from Docker labels
// Returns DepAnalysis for unified dependency handling
func AnalyzeComposeDeps(summary []container.Summary) DepAnalysis {
	result := DepAnalysis{
		Containers: make([]ContainerDepInfo, 0, len(summary)),
	}

	for _, c := range summary {
		serviceName := c.Labels["com.docker.compose.service"]
		if serviceName == "" {
			serviceName = cleanName(c.Names)
		}

		project := c.Labels["com.docker.compose.project"]
		if result.Project == "" && project != "" {
			result.Project = project
		}

		depsStr := c.Labels["com.docker.compose.depends_on"]
		var deps []string
		if depsStr != "" {
			deps = parseDependsOn(depsStr)
		}

		info := ContainerDepInfo{
			ContainerID:   c.ID,
			ContainerName: cleanName(c.Names),
			ServiceName:  serviceName,
			Project:     project,
			DependsOn:    deps,
		}
		result.Containers = append(result.Containers, info)
	}

	return result
}

func parseDependsOn(s string) []string {
	var result []string
	parts := strings.Split(s, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Docker Compose format: "service:condition:required"
		// Extract only the service name.
		if idx := strings.Index(p, ":"); idx > 0 {
			p = p[:idx]
		}
		result = append(result, p)
	}
	return result
}