// Package container defines the interface for interacting with container runtimes.
// The provider abstraction allows hived to work with Docker, Podman, or containerd
// on both Linux and Windows.
package container

import (
	"context"
	"io"
)

// Provider is the interface for container runtime operations.
// Implementations exist for Docker (which covers Podman via API compat).
type Provider interface {
	// RuntimeName returns the detected runtime ("docker", "podman", "containerd").
	RuntimeName() string

	// Ping verifies the container runtime is reachable.
	Ping(ctx context.Context) error

	// ListContainers returns all containers, optionally filtered by labels.
	ListContainers(ctx context.Context, filters map[string]string) ([]ContainerInfo, error)

	// CreateAndStart creates a container from the spec and starts it.
	CreateAndStart(ctx context.Context, spec ContainerSpec) (string, error)

	// Stop stops a running container with a timeout in seconds.
	Stop(ctx context.Context, id string, timeoutSeconds int) error

	// Remove removes a stopped container.
	Remove(ctx context.Context, id string) error

	// Logs returns a reader for container log output.
	Logs(ctx context.Context, id string, opts LogOpts) (io.ReadCloser, error)

	// Exec runs a command inside a running container.
	Exec(ctx context.Context, id string, cmd []string) (ExecResult, error)

	// PullImage pulls a container image from a registry.
	// auth may be nil for public images or when using default Docker credentials.
	PullImage(ctx context.Context, ref string, auth *RegistryAuth) error

	// Inspect returns detailed info about a container.
	Inspect(ctx context.Context, id string) (*ContainerInfo, error)

	// DetectCapabilities returns what platforms this runtime can run.
	DetectCapabilities() []string

	// CreateNetwork creates a Docker/Podman bridge network, returning its ID.
	CreateNetwork(ctx context.Context, name string) (string, error)

	// RemoveNetwork removes a network by name. Ignores "not found" errors.
	RemoveNetwork(ctx context.Context, name string) error

	// Stats returns live CPU and memory usage for a container (one-shot, non-streaming).
	Stats(ctx context.Context, id string) (*ContainerStats, error)

	// ListVolumes returns all named volumes on the host.
	ListVolumes(ctx context.Context) ([]VolumeInfo, error)

	// CreateVolume creates a named volume. Returns the mountpoint.
	CreateVolume(ctx context.Context, name string) (string, error)

	// DeleteVolume removes a named volume.
	DeleteVolume(ctx context.Context, name string) error

	// Close releases any resources held by the provider (e.g., HTTP client connections).
	Close() error
}

// ContainerStats holds per-container resource usage metrics.
type ContainerStats struct {
	CPUPercent  float64
	MemoryBytes uint64
}

// VolumeInfo holds metadata about a named volume.
type VolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	CreatedAt  string
}

// RegistryAuth holds credentials for authenticating to a container registry.
type RegistryAuth struct {
	Username string
	Password string
}

// ContainerSpec defines what to create.
type ContainerSpec struct {
	Name          string
	Image         string
	Env           map[string]string
	Ports         map[string]string // host:container
	Volumes       []VolumeSpec
	Labels        map[string]string
	MemoryMB      int64
	CPUs          float64
	Command       []string
	RestartPolicy  string   // "always", "on-failure", "no"
	NetworkName    string   // Docker network to attach to (empty = default bridge)
	NetworkAliases []string // DNS aliases on the Docker network
}

// VolumeSpec defines a volume mount.
type VolumeSpec struct {
	Name     string // named volume (empty = bind mount)
	Source   string // host path (for bind mounts)
	Target   string // container path
	ReadOnly bool
}

// ContainerInfo is the runtime state of a container.
type ContainerInfo struct {
	ID        string
	Name      string
	Image     string
	Status    string // "running", "stopped", "restarting"
	CreatedAt int64
	Labels    map[string]string
	Ports     map[string]string
}

// LogOpts configures log retrieval.
type LogOpts struct {
	Follow    bool
	TailLines int
	Since     string // RFC3339 or duration like "5m"
}

// ExecResult is the output of a command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}
