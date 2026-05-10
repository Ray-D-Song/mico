package cmd

import "github.com/ray-d-song/mico/pkg/core"

func sortServicesByDeps(services []core.Service) []core.Service {
	if len(services) <= 1 {
		return services
	}

	serviceByName := make(map[string]core.Service, len(services))
	inDegree := make(map[string]int, len(services))
	graph := make(map[string][]string, len(services))

	for _, svc := range services {
		serviceByName[svc.Name] = svc
		inDegree[svc.Name] = 0
	}

	for _, svc := range services {
		for _, dep := range svc.DependsOn {
			if _, ok := serviceByName[dep]; !ok {
				continue
			}
			graph[dep] = append(graph[dep], svc.Name)
			inDegree[svc.Name]++
		}
	}

	queue := make([]string, 0, len(services))
	for _, svc := range services {
		if inDegree[svc.Name] == 0 {
			queue = append(queue, svc.Name)
		}
	}

	result := make([]core.Service, 0, len(services))
	processed := make(map[string]bool, len(services))
	order := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if processed[current] {
			continue
		}

		svc := serviceByName[current]
		svc.StartOrder = order
		result = append(result, svc)
		processed[current] = true
		order++

		for _, next := range graph[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// Dependency cycles should not silently drop services from the migration.
	// Keep any cyclic remainder in its original order so unpack can still try to
	// restore everything and surface runtime errors if Docker cannot start it.
	for _, svc := range services {
		if processed[svc.Name] {
			continue
		}
		svc.StartOrder = order
		result = append(result, svc)
		order++
	}

	return result
}
