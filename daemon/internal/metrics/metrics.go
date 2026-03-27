// Package metrics provides Prometheus metrics for the Hive daemon.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// NodeCount tracks the number of nodes in the cluster.
	NodeCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "node_count",
		Help:      "Number of nodes in the cluster",
	})

	// ServiceCount tracks the number of deployed services.
	ServiceCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "service_count",
		Help:      "Number of deployed services",
	})

	// ContainerCount tracks running containers on this node.
	ContainerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "container_count",
		Help:      "Number of running containers on this node",
	})

	// HealthCheckTotal counts health checks by result.
	HealthCheckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hive",
		Name:      "health_check_total",
		Help:      "Total health checks by result",
	}, []string{"result"})

	// GRPCRequestsTotal counts gRPC requests by method.
	GRPCRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hive",
		Name:      "grpc_requests_total",
		Help:      "Total gRPC requests by method",
	}, []string{"method"})

	// DeployTotal counts deployments by status.
	DeployTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hive",
		Name:      "deploy_total",
		Help:      "Total deployments by status",
	}, []string{"status"})

	// SystemMemoryTotal is total system memory in bytes.
	SystemMemoryTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "system_memory_total_bytes",
		Help:      "Total system memory in bytes",
	})

	// SystemMemoryAvailable is available system memory in bytes.
	SystemMemoryAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "system_memory_available_bytes",
		Help:      "Available system memory in bytes",
	})

	// SystemDiskTotal is total disk space in bytes for the data directory.
	SystemDiskTotal = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "system_disk_total_bytes",
		Help:      "Total disk space in bytes",
	})

	// SystemDiskAvailable is available disk space in bytes.
	SystemDiskAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "hive",
		Name:      "system_disk_available_bytes",
		Help:      "Available disk space in bytes",
	})
)
