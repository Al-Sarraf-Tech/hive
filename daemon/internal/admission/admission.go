// Package admission provides a webhook-based admission controller for Hive.
// When configured, the webhook is called before deploy/update/scale operations
// and can reject or mutate the request.
package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
)

// Request is sent to the admission webhook before a deploy/update/scale.
type Request struct {
	Action   string                        `json:"action"` // "deploy", "update", "scale"
	Services map[string]hivefile.ServiceDef `json:"services"`
}

// Response is the webhook's answer.
type Response struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// CallWebhook sends an admission request to the configured webhook URL.
// Returns nil if allowed, error if denied or unreachable.
func CallWebhook(ctx context.Context, url string, req Request, timeout time.Duration) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal admission request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create admission request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("admission webhook unreachable: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*64))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admission webhook returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var admResp Response
	if err := json.Unmarshal(respBody, &admResp); err != nil {
		return fmt.Errorf("parse admission response: %w", err)
	}

	if !admResp.Allowed {
		reason := admResp.Reason
		if reason == "" {
			reason = "admission denied by webhook"
		}
		return fmt.Errorf("admission denied: %s", reason)
	}

	return nil
}
