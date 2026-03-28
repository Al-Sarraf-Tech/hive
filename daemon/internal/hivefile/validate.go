package hivefile

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jalsarraf0/hive/daemon/internal/cron"
)

// ValidationIssue represents a single problem found during hivefile validation.
type ValidationIssue struct {
	Severity string // "error", "warning", "info"
	Field    string // e.g. "service.web.health.port"
	Message  string
	Service  string // which service, empty for file-level
}

// Validate parses and validates a hivefile, returning all issues found.
// Returns a single parse-error issue if the TOML cannot be parsed at all.
func Validate(data string) []ValidationIssue {
	var issues []ValidationIssue

	hf, err := Parse([]byte(data))
	if err != nil {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Field:    "hivefile",
			Message:  fmt.Sprintf("parse error: %v", err),
		})
		return issues
	}

	// Deterministic iteration order for reproducible output
	svcNames := make([]string, 0, len(hf.Service))
	for name := range hf.Service {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)

	// Track host ports for collision detection: hostPort -> service name
	hostPorts := make(map[string]string)

	for _, name := range svcNames {
		svc := hf.Service[name]

		// Image is required
		if svc.Image == "" {
			issues = append(issues, ValidationIssue{
				Severity: "error",
				Field:    fmt.Sprintf("service.%s.image", name),
				Message:  "image is required",
				Service:  name,
			})
		}

		// Validate port values are numeric with valid range and detect collisions
		for hostPort, containerPort := range svc.Ports {
			// Validate host port is numeric and in range
			if hp, err := strconv.Atoi(hostPort); err != nil {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.ports", name),
					Message:  fmt.Sprintf("host port %q is not a valid number", hostPort),
					Service:  name,
				})
			} else if hp < 1 || hp > 65535 {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.ports", name),
					Message:  fmt.Sprintf("host port %d is out of range (1-65535)", hp),
					Service:  name,
				})
			}
			// Validate container port is numeric and in range
			if cp, err := strconv.Atoi(containerPort); err != nil {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.ports", name),
					Message:  fmt.Sprintf("container port %q is not a valid number", containerPort),
					Service:  name,
				})
			} else if cp < 1 || cp > 65535 {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.ports", name),
					Message:  fmt.Sprintf("container port %d is out of range (1-65535)", cp),
					Service:  name,
				})
			}
			// Check for host port collisions across services
			if prev, exists := hostPorts[hostPort]; exists {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.ports", name),
					Message:  fmt.Sprintf("host port %s is already used by service %q", hostPort, prev),
					Service:  name,
				})
			} else {
				hostPorts[hostPort] = name
			}
		}

		// Validate memory resource
		if svc.Resources.Memory != "" {
			if _, err := ParseMemory(svc.Resources.Memory); err != nil {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.resources.memory", name),
					Message:  fmt.Sprintf("invalid memory value: %v", err),
					Service:  name,
				})
			}
		}

		// Validate health check config
		if svc.Health.Type != "" {
			healthType := strings.ToLower(svc.Health.Type)
			switch healthType {
			case "http", "tcp", "exec":
				// valid
			default:
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.health.type", name),
					Message:  fmt.Sprintf("invalid health check type %q, must be one of: http, tcp, exec", svc.Health.Type),
					Service:  name,
				})
			}

			if svc.Health.Port <= 0 || svc.Health.Port > 65535 {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.health.port", name),
					Message:  "health check port must be > 0 when type is set",
					Service:  name,
				})
			}

			if svc.Health.Path != "" && healthType != "http" {
				issues = append(issues, ValidationIssue{
					Severity: "warning",
					Field:    fmt.Sprintf("service.%s.health.path", name),
					Message:  "health check path is only used for http type checks",
					Service:  name,
				})
			}
		}

		// Validate cron schedules
		for i, cronDef := range svc.Cron {
			if _, err := cron.Parse(cronDef.Schedule); err != nil {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.cron[%d].schedule", name, i),
					Message:  fmt.Sprintf("invalid cron expression: %v", err),
					Service:  name,
				})
			}
		}

		// Validate deploy strategy
		if svc.Deploy.Strategy != "" {
			switch svc.Deploy.Strategy {
			case "rolling", "recreate", "blue-green":
				// valid
			default:
				issues = append(issues, ValidationIssue{
					Severity: "error",
					Field:    fmt.Sprintf("service.%s.deploy.strategy", name),
					Message:  fmt.Sprintf("invalid deploy strategy %q, must be one of: rolling, recreate, blue-green", svc.Deploy.Strategy),
					Service:  name,
				})
			}
		}

		// Warn if no health check configured
		if svc.Health.Type == "" {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				Field:    fmt.Sprintf("service.%s.health", name),
				Message:  "no health check configured",
				Service:  name,
			})
		}

		// Warn if replicas > 1 but no health check
		if svc.Replicas > 1 && svc.Health.Type == "" {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				Field:    fmt.Sprintf("service.%s", name),
				Message:  fmt.Sprintf("replicas=%d but no health check configured — failures may go undetected", svc.Replicas),
				Service:  name,
			})
		}
	}

	// Check for dependency cycles via topological sort
	if _, err := TopoSort(hf.Service); err != nil {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Field:    "depends_on",
			Message:  fmt.Sprintf("dependency error: %v", err),
		})
	}

	return issues
}
