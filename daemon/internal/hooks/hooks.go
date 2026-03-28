// Package hooks delivers webhook notifications for lifecycle events.
package hooks

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Manager dispatches lifecycle events to configured webhook URLs.
type Manager struct {
	hooks  []Hook
	client *http.Client
}

// Hook maps an event type to a webhook URL.
type Hook struct {
	Type string // "pre-deploy", "post-deploy", "pre-stop", "health-fail", or "*" for all
	URL  string
}

// Event is the JSON payload delivered to webhook endpoints.
type Event struct {
	Type    string `json:"type"`
	Service string `json:"service,omitempty"`
	Node    string `json:"node,omitempty"`
	Message string `json:"message,omitempty"`
	Time    time.Time `json:"time"`
}

// NewManager creates a hook manager. If hooks is empty, Fire is a no-op.
func NewManager(hooks []Hook) *Manager {
	return &Manager{
		hooks:  hooks,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fire dispatches an event to all hooks matching the event type (or wildcard "*").
func (m *Manager) Fire(eventType string, event Event) {
	if m == nil {
		return
	}
	event.Type = eventType
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	for _, h := range m.hooks {
		if h.Type == eventType || h.Type == "*" {
			go m.send(h.URL, event)
		}
	}
}

func (m *Manager) send(url string, event Event) {
	body, err := json.Marshal(event)
	if err != nil {
		slog.Warn("webhook: failed to marshal event", "url", url, "error", err)
		return
	}
	resp, err := m.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("webhook delivery failed", "url", url, "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("webhook returned error", "url", url, "status", resp.StatusCode)
	}
}
