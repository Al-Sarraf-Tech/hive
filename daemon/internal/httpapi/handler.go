// Package httpapi provides a lightweight HTTP/JSON gateway that wraps HiveAPI gRPC calls.
// Designed for the web console — no grpc-gateway dependency required.
package httpapi

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/joincode"
	"github.com/jalsarraf0/hive/daemon/internal/logs"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler wraps a HiveAPI gRPC client to serve HTTP/JSON endpoints.
type Handler struct {
	api       hivev1.HiveAPIServer
	mux       *http.ServeMux
	token     string           // bearer token for authentication (empty = no auth)
	logBuffer *logs.RingBuffer // nil if log aggregation is disabled
	store     *store.Store     // direct store access for bootstrap endpoint
	dataDir   string           // data directory for reading CA cert
}

// New creates an HTTP handler that delegates to the given gRPC API server.
func New(api hivev1.HiveAPIServer, token string, logBuffer *logs.RingBuffer, s *store.Store, dataDir string) *Handler {
	h := &Handler{api: api, mux: http.NewServeMux(), token: token, logBuffer: logBuffer, store: s, dataDir: dataDir}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for browser console.
	// CORS policy:
	// - GET/OPTIONS: allow any origin (read-only, safe for public access)
	// - POST/DELETE: NO Access-Control-Allow-Origin header = browser enforces same-origin
	//   This blocks cross-site requests to mutation endpoints (deploy, exec, secrets, etc.)
	//   The console is served from the same host:port so it works without CORS.
	//   CLI/API tools (curl, gRPC) are unaffected since they don't enforce CORS.
	if r.Method == http.MethodOptions || r.Method == http.MethodGet {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	// Only advertise safe methods in preflight; mutations require same-origin
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Bearer token authentication (skip for OPTIONS already handled above)
	// /metrics and /api/v1/bootstrap/ are unauthenticated
	if h.token != "" && !strings.HasPrefix(r.URL.Path, "/metrics") && !strings.HasPrefix(r.URL.Path, "/api/v1/bootstrap/") {
		auth := r.Header.Get("Authorization")
		// Also accept token as query parameter (needed for SSE — EventSource cannot set headers)
		if auth == "" {
			if qToken := r.URL.Query().Get("token"); qToken != "" {
				auth = "Bearer " + qToken
			}
		}
		expected := "Bearer " + h.token
		if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized — provide Authorization: Bearer <token>"}`))
			return
		}
	}

	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	// Bootstrap endpoint is unauthenticated — must be registered before auth middleware applies.
	h.mux.HandleFunc("GET /api/v1/bootstrap/{code}", h.bootstrap)

	h.mux.Handle("GET /metrics", promhttp.Handler())
	h.mux.HandleFunc("GET /api/v1/logs/stream", h.streamLogs)
	h.mux.HandleFunc("GET /api/v1/logs", h.getLogs)
	h.mux.HandleFunc("GET /api/v1/logs/{service}", h.getServiceLogs)
	h.mux.HandleFunc("GET /api/v1/status", h.getStatus)
	h.mux.HandleFunc("GET /api/v1/nodes", h.listNodes)
	h.mux.HandleFunc("GET /api/v1/services", h.listServices)
	h.mux.HandleFunc("GET /api/v1/containers", h.listContainers)
	h.mux.HandleFunc("GET /api/v1/secrets", h.listSecrets)
	h.mux.HandleFunc("POST /api/v1/deploy", h.deploy)
	h.mux.HandleFunc("POST /api/v1/services/{name}/stop", h.stopService)
	h.mux.HandleFunc("POST /api/v1/services/{name}/scale", h.scaleService)
	h.mux.HandleFunc("POST /api/v1/services/{name}/rollback", h.rollbackService)
	h.mux.HandleFunc("POST /api/v1/services/{name}/exec", h.execContainer)
	h.mux.HandleFunc("POST /api/v1/services/{name}/restart", h.restartService)
	h.mux.HandleFunc("POST /api/v1/nodes/{name}/drain", h.drainNode)
	h.mux.HandleFunc("POST /api/v1/secrets/{key}", h.setSecret)
	h.mux.HandleFunc("DELETE /api/v1/secrets/{key}", h.deleteSecret)
	h.mux.HandleFunc("GET /api/v1/cron", h.listCronJobs)
	h.mux.HandleFunc("GET /api/v1/volumes", h.listVolumes)
	h.mux.HandleFunc("POST /api/v1/volumes", h.createVolume)
	h.mux.HandleFunc("DELETE /api/v1/volumes/{name}", h.deleteVolume)
	h.mux.HandleFunc("POST /api/v1/validate", h.validateHivefile)
	h.mux.HandleFunc("POST /api/v1/diff", h.diffDeploy)
	h.mux.HandleFunc("GET /api/v1/services/{name}/health", h.getServiceHealth)
}

func (h *Handler) getLogs(w http.ResponseWriter, r *http.Request) {
	if h.logBuffer == nil {
		writeJSON(w, []logs.Entry{})
		return
	}
	n := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 10000 {
		n = 10000
	}
	writeJSON(w, h.logBuffer.Last(n))
}

func (h *Handler) getServiceLogs(w http.ResponseWriter, r *http.Request) {
	if h.logBuffer == nil {
		writeJSON(w, []logs.Entry{})
		return
	}
	service := r.PathValue("service")
	n := 200
	if q := r.URL.Query().Get("lines"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 10000 {
		n = 10000
	}
	writeJSON(w, h.logBuffer.ForService(service, n))
}

func (h *Handler) streamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	if h.logBuffer == nil {
		jsonError(w, "log aggregation disabled", http.StatusNotFound)
		return
	}

	// Disable the server's WriteTimeout for this long-lived SSE connection.
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	service := r.URL.Query().Get("service")
	lastID := 0

	for {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
			entries := h.logBuffer.Since(lastID, service)
			for _, e := range entries {
				data, _ := json.Marshal(e)
				fmt.Fprintf(w, "data: %s\n\n", data)
				lastID = e.ID
			}
			if len(entries) > 0 {
				flusher.Flush()
			}
		}
	}
}

func (h *Handler) getStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.GetClusterStatus(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListNodes(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListServices(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) listContainers(w http.ResponseWriter, r *http.Request) {
	svcName := r.URL.Query().Get("service")
	nodeName := r.URL.Query().Get("node")
	resp, err := h.api.ListContainers(r.Context(), &hivev1.ListContainersRequest{
		ServiceName: svcName,
		NodeName:    nodeName,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) listSecrets(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListSecrets(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) deploy(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit for hivefiles
	var body struct {
		HivefileToml string `json:"hivefile_toml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	resp, err := h.api.DeployService(r.Context(), &hivev1.DeployServiceRequest{
		HivefileToml: body.HivefileToml,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) stopService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := h.api.StopService(r.Context(), &hivev1.StopServiceRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "stopped", "service": name})
}

func (h *Handler) scaleService(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	name := r.PathValue("name")
	var body struct {
		Replicas uint32 `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	_, err := h.api.ScaleService(r.Context(), &hivev1.ScaleServiceRequest{
		Name:     name,
		Replicas: body.Replicas,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]any{"status": "scaled", "service": name, "replicas": body.Replicas})
}

func (h *Handler) rollbackService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := h.api.RollbackService(r.Context(), &hivev1.RollbackServiceRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "rolled_back", "service": name})
}

func (h *Handler) execContainer(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	name := r.PathValue("name")
	var body struct {
		Command []string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if len(body.Command) == 0 {
		jsonError(w, "command must not be empty", http.StatusBadRequest)
		return
	}
	resp, err := h.api.ExecContainer(r.Context(), &hivev1.ExecContainerRequest{
		ServiceName: name,
		Command:     body.Command,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) restartService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := h.api.RestartService(r.Context(), &hivev1.RestartServiceRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "restarted", "service": name})
}

func (h *Handler) drainNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := h.api.DrainNode(r.Context(), &hivev1.DrainNodeRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "drained", "node": name})
}

func (h *Handler) setSecret(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	key := r.PathValue("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	_, err := h.api.SetSecret(r.Context(), &hivev1.SetSecretRequest{
		Key:   key,
		Value: []byte(body.Value),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "set", "key": key})
}

func (h *Handler) deleteSecret(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	_, err := h.api.DeleteSecret(r.Context(), &hivev1.DeleteSecretRequest{Key: key})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "key": key})
}

func (h *Handler) listCronJobs(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListCronJobs(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) listVolumes(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ListVolumes(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) createVolume(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "volume name is required", http.StatusBadRequest)
		return
	}
	resp, err := h.api.CreateVolume(r.Context(), &hivev1.CreateVolumeRequest{Name: body.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) deleteVolume(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, err := h.api.DeleteVolume(r.Context(), &hivev1.DeleteVolumeRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "volume": name})
}

func (h *Handler) validateHivefile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit for hivefiles
	var body struct {
		HivefileToml string `json:"hivefile_toml"`
		ServerChecks bool   `json:"server_checks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	resp, err := h.api.ValidateHivefile(r.Context(), &hivev1.ValidateHivefileRequest{
		HivefileToml: body.HivefileToml,
		ServerChecks: body.ServerChecks,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) diffDeploy(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit for hivefiles
	var body struct {
		HivefileToml string `json:"hivefile_toml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	resp, err := h.api.DiffDeploy(r.Context(), &hivev1.DiffDeployRequest{
		HivefileToml: body.HivefileToml,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) getServiceHealth(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var limit int32
	if q := r.URL.Query().Get("limit"); q != "" {
		if parsed, err := strconv.Atoi(q); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}
	resp, err := h.api.GetServiceHealth(r.Context(), &hivev1.GetServiceHealthRequest{
		Name:  name,
		Limit: limit,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) exportCluster(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ExportCluster(r.Context(), &emptypb.Empty{})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) importCluster(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50MB limit for backups
	var body struct {
		Data      []byte `json:"data"`
		Overwrite bool   `json:"overwrite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	resp, err := h.api.ImportCluster(r.Context(), &hivev1.ImportClusterRequest{
		Data:      body.Data,
		Overwrite: body.Overwrite,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

// jsonError sends a JSON-encoded error response with the given HTTP status code.
// Unlike http.Error(), this sets Content-Type to application/json so the console
// UI can parse error responses consistently.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// writeProto serializes a protobuf message as JSON using protojson.
func writeProto(w http.ResponseWriter, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	data, err := protojson.Marshal(msg)
	if err != nil {
		jsonError(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

// writeJSON writes a Go value as JSON.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// writeError maps gRPC errors to HTTP status codes.
func writeError(w http.ResponseWriter, err error) {
	st, ok := grpcstatus.FromError(err)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"})
		return
	}

	httpCode := http.StatusInternalServerError
	switch st.Code() {
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.Unimplemented:
		httpCode = http.StatusNotImplemented
	case codes.PermissionDenied:
		httpCode = http.StatusForbidden
	case codes.FailedPrecondition:
		httpCode = http.StatusPreconditionFailed
	case codes.AlreadyExists:
		httpCode = http.StatusConflict
	case codes.Canceled:
		httpCode = http.StatusInternalServerError // 408 is "server timed out waiting for request", not appropriate for cancelled operations
	case codes.DeadlineExceeded:
		httpCode = http.StatusGatewayTimeout
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	json.NewEncoder(w).Encode(map[string]string{"error": st.Message()})
}

// bootstrapLimiter tracks per-IP request counts for rate limiting the bootstrap endpoint.
var bootstrapLimiter struct {
	mu      sync.Mutex
	counts  map[string]int
	resetAt time.Time
}

func init() {
	bootstrapLimiter.counts = make(map[string]int)
	bootstrapLimiter.resetAt = time.Now().Add(time.Minute)
}

// checkBootstrapRate returns true if the request is allowed (under rate limit).
func checkBootstrapRate(ip string) bool {
	bootstrapLimiter.mu.Lock()
	defer bootstrapLimiter.mu.Unlock()
	if time.Now().After(bootstrapLimiter.resetAt) {
		bootstrapLimiter.counts = make(map[string]int)
		bootstrapLimiter.resetAt = time.Now().Add(time.Minute)
	}
	bootstrapLimiter.counts[ip]++
	return bootstrapLimiter.counts[ip] <= 5 // 5 attempts per minute per IP
}

// bootstrap handles unauthenticated join-code exchange: given a valid short code,
// it returns the full join token, gossip address, and CA certificate so a new node
// can join with zero pre-shared secrets beyond the code itself.
func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	// Rate limit: 5 attempts per minute per IP
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip == "" {
		ip = r.RemoteAddr
	}
	if !checkBootstrapRate(ip) {
		jsonError(w, "rate limit exceeded — try again in 1 minute", http.StatusTooManyRequests)
		return
	}

	if h.store == nil {
		jsonError(w, "store not available", http.StatusInternalServerError)
		return
	}

	// Look up the stored join code.
	storedCode, err := h.store.Get("meta", "join_code")
	if err != nil || storedCode == nil {
		jsonError(w, "no cluster initialized", http.StatusNotFound)
		return
	}

	// Normalize and compare using constant-time comparison.
	normalizedInput, err := joincode.Decode(code)
	if err != nil {
		// Add brief delay on invalid attempts to slow brute-force
		time.Sleep(500 * time.Millisecond)
		jsonError(w, "invalid join code format", http.StatusBadRequest)
		return
	}
	normalizedStored, err := joincode.Decode(string(storedCode))
	if err != nil {
		jsonError(w, "internal error: stored join code is corrupt", http.StatusInternalServerError)
		return
	}
	if subtle.ConstantTimeCompare([]byte(normalizedInput), []byte(normalizedStored)) != 1 {
		time.Sleep(500 * time.Millisecond)
		jsonError(w, "invalid join code", http.StatusUnauthorized)
		return
	}

	// Retrieve the full token and gossip address — validate they exist.
	token, err := h.store.Get("meta", "join_token")
	if err != nil || token == nil {
		jsonError(w, "join token not found", http.StatusInternalServerError)
		return
	}
	addr, err := h.store.Get("meta", "join_code_addr")
	if err != nil || addr == nil {
		jsonError(w, "gossip address not found", http.StatusInternalServerError)
		return
	}
	clusterName, _ := h.store.Get("meta", "cluster_name")

	// Read CA certificate from disk.
	caCertPEM, err := os.ReadFile(filepath.Join(h.dataDir, "pki", "ca.crt"))
	if err != nil {
		slog.Warn("bootstrap: CA cert not available", "error", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"join_token":   string(token),
		"gossip_addr":  string(addr),
		"ca_cert_pem":  string(caCertPEM),
		"cluster_name": string(clusterName),
	})
}

// NewServer creates an *http.Server with timeouts, ready for graceful shutdown.
// If tlsConfig is non-nil, the server's TLSConfig is set so the caller can use
// ListenAndServeTLS("", "") with certificates provided via GetCertificate.
func NewServer(addr string, api hivev1.HiveAPIServer, token string, logBuffer *logs.RingBuffer, s *store.Store, dataDir string, tlsConfig *tls.Config) *http.Server {
	h := New(api, token, logBuffer, s, dataDir)
	srv := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if tlsConfig != nil {
		srv.TLSConfig = tlsConfig
	}
	return srv
}
