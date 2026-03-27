package hivefile

import "fmt"

// TopoSort returns service names in dependency order (dependencies first).
// Uses Kahn's algorithm. Returns an error if there is a cycle or if a
// depends_on references a service not defined in the hivefile.
func TopoSort(services map[string]ServiceDef) ([]string, error) {
	// Build adjacency list and in-degree count
	inDegree := make(map[string]int, len(services))
	dependents := make(map[string][]string, len(services)) // dep -> []services that depend on it

	for name := range services {
		inDegree[name] = 0
	}

	for name, svc := range services {
		for _, dep := range svc.DependsOn.Services {
			if _, ok := services[dep]; !ok {
				return nil, fmt.Errorf("service %q depends on %q, which is not defined in the hivefile", name, dep)
			}
			dependents[dep] = append(dependents[dep], name)
			inDegree[name]++
		}
	}

	// Seed the queue with services that have no dependencies
	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	// Stable sort of the initial queue for deterministic output
	sortStrings(queue)

	var order []string
	for len(queue) > 0 {
		// Pop front
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		// For each service that depends on current, decrement in-degree
		deps := dependents[current]
		sortStrings(deps) // stable ordering within each level
		for _, dep := range deps {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(order) != len(services) {
		// Find services involved in the cycle for a useful error message
		var cycled []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycled = append(cycled, name)
			}
		}
		sortStrings(cycled)
		return nil, fmt.Errorf("dependency cycle detected among services: %v", cycled)
	}

	return order, nil
}

// sortStrings sorts a string slice in place (insertion sort — fine for small N).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
