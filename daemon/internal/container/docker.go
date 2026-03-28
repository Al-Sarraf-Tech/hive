package container

import (
	"context"
	"encoding/binary"
	"encoding/json"
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

const maxExecOutput = 10 * 1024 * 1024 // 10 MB max exec output per stream

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
		ports := make(map[string]string)
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				ports[fmt.Sprintf("%d", p.PublicPort)] = fmt.Sprintf("%d", p.PrivatePort)
			}
		}
		out = append(out, ContainerInfo{
			ID:        c.ID,
			Name:      cname,
			Image:     c.Image,
			Status:    string(c.State),
			CreatedAt: c.Created,
			Labels:    c.Labels,
			Ports:     ports,
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

	var networkingCfg *network.NetworkingConfig
	if spec.NetworkName != "" {
		hostCfg.NetworkMode = container.NetworkMode(spec.NetworkName)
		networkingCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				spec.NetworkName: {
					Aliases: spec.NetworkAliases,
				},
			},
		}
	}

	resp, err := d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           cfg,
		HostConfig:       hostCfg,
		NetworkingConfig: networkingCfg,
		Name:             spec.Name,
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
	// Enforce a 5-minute timeout on exec operations (HIGH 1.2)
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	exec, err := d.cli.ExecCreate(execCtx, id, client.ExecCreateOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return ExecResult{}, fmt.Errorf("create exec: %w", err)
	}

	resp, err := d.cli.ExecAttach(execCtx, exec.ID, client.ExecAttachOptions{})
	if err != nil {
		return ExecResult{}, fmt.Errorf("attach exec: %w", err)
	}
	defer resp.Close()

	// Docker exec attach returns a multiplexed stream — demux stdout/stderr
	var stdout, stderr strings.Builder
	if err := demuxDockerStream(resp.Reader, &stdout, &stderr); err != nil {
		return ExecResult{}, fmt.Errorf("read exec output: %w", err)
	}

	// Get exit code
	inspect, err := d.cli.ExecInspect(execCtx, exec.ID, client.ExecInspectOptions{})
	if err != nil {
		return ExecResult{ExitCode: -1, Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}

	return ExecResult{
		ExitCode: inspect.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// demuxDockerStream reads a Docker multiplexed stream and writes stdout/stderr
// to the respective writers. The Docker stream protocol uses an 8-byte header
// per frame: [stream_type(1), padding(3), size(4 big-endian)].
// Stream types: 0=stdin, 1=stdout, 2=stderr.
// Output is capped at maxExecOutput bytes total to prevent OOM (HIGH 1.1).
func demuxDockerStream(r io.Reader, stdout, stderr io.Writer) error {
	header := make([]byte, 8)
	var totalBytes int64
	for {
		if _, err := io.ReadFull(r, header); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		streamType := header[0]
		frameSize := binary.BigEndian.Uint32(header[4:8])
		if frameSize == 0 {
			continue
		}

		// Check if this frame would exceed the output size limit
		if totalBytes+int64(frameSize) > maxExecOutput {
			// Drain remaining data without storing it
			_, _ = io.CopyN(io.Discard, r, int64(frameSize))
			return fmt.Errorf("exec output truncated: exceeded %d byte limit", maxExecOutput)
		}

		var dst io.Writer
		switch streamType {
		case 1:
			dst = stdout
		case 2:
			dst = stderr
		default:
			dst = stdout // treat stdin/unknown as stdout
		}

		n, err := io.CopyN(dst, r, int64(frameSize))
		totalBytes += n
		if err != nil {
			return err
		}
	}
}

func (d *dockerProvider) Stats(ctx context.Context, id string) (*ContainerStats, error) {
	resp, err := d.cli.ContainerStats(ctx, id, client.ContainerStatsOptions{
		Stream:                false, // one-shot
		IncludePreviousSample: true,  // populate PreCPUStats for delta calculation
	})
	if err != nil {
		return nil, fmt.Errorf("get container stats: %w", err)
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode container stats: %w", err)
	}

	// Calculate CPU percentage from the delta between current and previous sample
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
	}

	return &ContainerStats{
		CPUPercent:  cpuPercent,
		MemoryBytes: stats.MemoryStats.Usage,
	}, nil
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
	status := ""
	if info.State != nil {
		status = string(info.State.Status)
	}
	return &ContainerInfo{
		ID:     info.ID,
		Name:   strings.TrimPrefix(info.Name, "/"),
		Image:  info.Config.Image,
		Status: status,
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

func (d *dockerProvider) CreateNetwork(ctx context.Context, name string) (string, error) {
	resp, err := d.cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver: "bridge",
		Labels: map[string]string{"hive.managed": "true"},
	})
	if err != nil {
		// Idempotent: if network already exists, return its ID
		if strings.Contains(err.Error(), "already exists") {
			existing, inspErr := d.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
			if inspErr != nil {
				return "", fmt.Errorf("network %q exists but inspect failed: %w", name, inspErr)
			}
			return existing.Network.ID, nil
		}
		return "", fmt.Errorf("create network %q: %w", name, err)
	}
	return resp.ID, nil
}

func (d *dockerProvider) RemoveNetwork(ctx context.Context, name string) error {
	_, err := d.cli.NetworkRemove(ctx, name, client.NetworkRemoveOptions{})
	if err != nil && strings.Contains(err.Error(), "not found") {
		return nil // already gone
	}
	return err
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
