// Package health implements health checking for services.
package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/container"
)

// CheckType defines the type of health check.
type CheckType string

const (
	CheckHTTP CheckType = "http"
	CheckTCP  CheckType = "tcp"
	CheckExec CheckType = "exec"
)

// Config defines a health check configuration.
type Config struct {
	Type        CheckType
	Host        string
	Port        int
	Path        string        // for HTTP checks
	Interval    time.Duration
	Timeout     time.Duration
	Retries     int
	ContainerID string   // container ID for exec-type checks
	ExecCommand []string // command to run for exec-type checks
}

// Result is the outcome of a health check.
type Result struct {
	Healthy   bool
	Message   string
	CheckedAt time.Time
	Duration  time.Duration
}

// Checker runs health checks against services.
type Checker struct {
	httpClient *http.Client
	container  container.Provider // for exec-type health checks
}

// NewChecker creates a new health checker.
// The container provider is used for exec-type health checks (may be nil if exec checks are not needed).
func NewChecker(cp container.Provider) *Checker {
	return &Checker{
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // safety-net timeout in case per-request context is not propagated
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse // don't follow redirects
			},
		},
		container: cp,
	}
}

// Check runs a single health check based on the config.
func (c *Checker) Check(ctx context.Context, cfg Config) Result {
	start := time.Now()

	var result Result
	switch cfg.Type {
	case CheckHTTP:
		result = c.checkHTTP(ctx, cfg)
	case CheckTCP:
		result = c.checkTCP(ctx, cfg)
	case CheckExec:
		if cfg.ContainerID == "" {
			result = Result{Healthy: false, Message: "no container ID for exec check"}
		} else {
			result = c.checkExec(ctx, cfg)
		}
	default:
		result = Result{
			Healthy: false,
			Message: fmt.Sprintf("unknown check type: %s", cfg.Type),
		}
	}

	result.CheckedAt = start
	result.Duration = time.Since(start)

	slog.Debug("health check completed",
		"type", cfg.Type,
		"host", cfg.Host,
		"port", cfg.Port,
		"healthy", result.Healthy,
		"duration", result.Duration,
	)

	return result
}

func (c *Checker) checkHTTP(ctx context.Context, cfg Config) Result {
	path := cfg.Path
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	url := fmt.Sprintf("http://%s:%d%s", cfg.Host, cfg.Port, path)

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, url, nil)
	if err != nil {
		return Result{Healthy: false, Message: fmt.Sprintf("create request: %v", err)}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{Healthy: false, Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body) // drain body for connection reuse
		resp.Body.Close()
	}()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{Healthy: true, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Result{Healthy: false, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

func (c *Checker) checkTCP(ctx context.Context, cfg Config) Result {
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Result{Healthy: false, Message: fmt.Sprintf("tcp connect failed: %v", err)}
	}
	conn.Close()
	return Result{Healthy: true, Message: "tcp connect ok"}
}

func (c *Checker) checkExec(ctx context.Context, cfg Config) Result {
	if c.container == nil {
		return Result{Healthy: false, Message: "container provider not available for exec check", CheckedAt: time.Now()}
	}
	start := time.Now()
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := c.container.Exec(execCtx, cfg.ContainerID, cfg.ExecCommand)
	duration := time.Since(start)
	if err != nil {
		return Result{Healthy: false, Message: fmt.Sprintf("exec error: %v", err), CheckedAt: start, Duration: duration}
	}
	if result.ExitCode != 0 {
		msg := fmt.Sprintf("exit code %d", result.ExitCode)
		if result.Stderr != "" {
			msg += ": " + result.Stderr
		}
		return Result{Healthy: false, Message: msg, CheckedAt: start, Duration: duration}
	}
	return Result{Healthy: true, Message: "exec ok", CheckedAt: start, Duration: duration}
}
