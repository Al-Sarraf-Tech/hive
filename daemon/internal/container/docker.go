package container

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// dockerProvider implements Provider using the Docker Engine API.
// Works with Docker Desktop (Windows/Linux), Podman (via compat API), and
// native Docker Engine. Transport is auto-detected:
//   - Linux: unix:///var/run/docker.sock
//   - Windows: npipe:////./pipe/docker_engine
//   - Or DOCKER_HOST env var
type dockerProvider struct {
	cli         *client.Client
	runtimeName string
}

// NewDockerProvider creates a container provider that talks to Docker/Podman.
func NewDockerProvider() (Provider, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ping, err := cli.Ping(ctx, client.PingOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to container runtime: %w", err)
	}

	name := "docker"
	if strings.Contains(strings.ToLower(ping.APIVersion), "libpod") {
		name = "podman"
	}

	slog.Info("connected to container runtime",
		"runtime", name,
		"api_version", ping.APIVersion,
		"os", runtime.GOOS,
	)

	return &dockerProvider{cli: cli, runtimeName: name}, nil
}

// ShortID safely truncates a container ID to at most 12 characters for display.
// Use only for logging and user-facing output — never for API operations.
func ShortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (d *dockerProvider) RuntimeName() string {
	return d.runtimeName
}

func (d *dockerProvider) Ping(ctx context.Context) error {
	_, err := d.cli.Ping(ctx, client.PingOptions{})
	return err
}

func (d *dockerProvider) ListContainers(ctx context.Context, filters map[string]string) ([]ContainerInfo, error) {
	result, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	var out []ContainerInfo
	for _, c := range result.Items {
		if !matchLabels(c.Labels, filters) {
			continue
		}
		cname := ""
		if len(c.Names) > 0 {
			cname = strings.TrimPrefix(c.Names[0], "/")
		}
		out = append(out, ContainerInfo{
			ID:        c.ID,
			Name:      cname,
			Image:     c.Image,
			Status:    string(c.State),
			CreatedAt: c.Created,
			Labels:    c.Labels,
		})
	}
	return out, nil
}

func (d *dockerProvider) CreateAndStart(ctx context.Context, spec ContainerSpec) (string, error) {
	// Build port bindings — validate port values before using MustParsePort
	portBindings := network.PortMap{}
	exposedPorts := network.PortSet{}
	for hostPort, containerPort := range spec.Ports {
		cp, err := network.ParsePort(containerPort + "/tcp")
		if err != nil {
			return "", fmt.Errorf("invalid container port %q: %w", containerPort, err)
		}
		exposedPorts[cp] = struct{}{}
		portBindings[cp] = []network.PortBinding{{HostPort: hostPort}}
	}

	// Build env slice
	var env []string
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	// Build mounts — skip volumes with no source on this platform
	var binds []string
	for _, v := range spec.Volumes {
		if v.Name == "" && v.Source == "" {
			continue // no source available (e.g., only Windows path on Linux)
		}
		var bind string
		if v.Name != "" && v.Source == "" {
			bind = v.Name + ":" + v.Target
		} else {
			bind = v.Source + ":" + v.Target
		}
		if v.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	// Build labels — tag all hive-managed containers
	labels := map[string]string{
		"hive.managed": "true",
	}
	for k, v := range spec.Labels {
		labels[k] = v
	}

	cfg := &container.Config{
		Image:        spec.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Labels:       labels,
		Cmd:          spec.Command,
	}

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        binds,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(spec.RestartPolicy),
		},
	}

	if spec.MemoryMB > 0 {
		hostCfg.Resources.Memory = spec.MemoryMB * 1024 * 1024
	}
	if spec.CPUs > 0 {
		hostCfg.Resources.NanoCPUs = int64(spec.CPUs * 1e9)
	}

	resp, err := d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
		Name:       spec.Name,
	})
	if err != nil {
		return "", fmt.Errorf("create container %q: %w", spec.Name, err)
	}

	if _, err := d.cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		// Clean up the created-but-not-started container to avoid orphans
		_, _ = d.cli.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		return "", fmt.Errorf("start container %q: %w", spec.Name, err)
	}

	slog.Info("container started", "name", spec.Name, "id", ShortID(resp.ID))
	return resp.ID, nil
}

func (d *dockerProvider) Stop(ctx context.Context, id string, timeoutSeconds int) error {
	timeout := timeoutSeconds
	_, err := d.cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout})
	return err
}

func (d *dockerProvider) Remove(ctx context.Context, id string) error {
	_, err := d.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	return err
}

func (d *dockerProvider) Logs(ctx context.Context, id string, opts LogOpts) (io.ReadCloser, error) {
	tail := "all"
	if opts.TailLines > 0 {
		tail = fmt.Sprintf("%d", opts.TailLines)
	}
	return d.cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       tail,
	})
}

func (d *dockerProvider) Exec(ctx context.Context, id string, cmd []string) (ExecResult, error) {
	return ExecResult{}, fmt.Errorf("exec not yet implemented")
}

func (d *dockerProvider) PullImage(ctx context.Context, ref string) error {
	resp, err := d.cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %q: %w", ref, err)
	}
	defer resp.Close()
	_, err = io.Copy(io.Discard, resp)
	return err
}

func (d *dockerProvider) Inspect(ctx context.Context, id string) (*ContainerInfo, error) {
	result, err := d.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect container %q: %w", id, err)
	}
	info := result.Container
	return &ContainerInfo{
		ID:     info.ID,
		Name:   strings.TrimPrefix(info.Name, "/"),
		Image:  info.Config.Image,
		Status: string(info.State.Status),
		Labels: info.Config.Labels,
	}, nil
}

func (d *dockerProvider) DetectCapabilities() []string {
	caps := []string{runtime.GOOS + "/" + runtime.GOARCH}
	if runtime.GOOS == "windows" {
		caps = append(caps, "linux/"+runtime.GOARCH)
	}
	return caps
}

func (d *dockerProvider) Close() error {
	return d.cli.Close()
}

func matchLabels(containerLabels, filters map[string]string) bool {
	for k, v := range filters {
		if containerLabels[k] != v {
			return false
		}
	}
	return true
}
