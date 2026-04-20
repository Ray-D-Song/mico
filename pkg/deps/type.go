package deps

// ContainerDepInfo holds all dependency information for a single container
type ContainerDepInfo struct {
	ContainerID   string
	ContainerName string
	ServiceName  string
	Project      string
	DependsOn    []string
	Networks     []string
}

// DepAnalysis is the unified result containing dependency analysis for all containers
type DepAnalysis struct {
	Containers []ContainerDepInfo
	Project    string
}

// cleanName removes leading slash from container name
func cleanName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	name := names[0]
	if len(name) > 0 && name[0] == '/' {
		return name[1:]
	}
	return name
}