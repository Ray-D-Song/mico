package deps

type ServiceDep struct {
	ServiceName string
	DependsOn   []string
}

type DepGraph struct {
	Services []ServiceDep
	Project  string
}
