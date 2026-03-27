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
	Type     CheckType
	Host     string
	Port     int
	Path     string        // for HTTP checks
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
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
}

// NewChecker creates a new health checker.
func NewChecker() *Checker {
	return &Checker{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse // don't follow redirects
			},
		},
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
