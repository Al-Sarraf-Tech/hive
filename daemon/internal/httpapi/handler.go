// Package httpapi provides a lightweight HTTP/JSON gateway that wraps HiveAPI gRPC calls.
// Designed for the web console — no grpc-gateway dependency required.
package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/logs"
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
}

// New creates an HTTP handler that delegates to the given gRPC API server.
func New(api hivev1.HiveAPIServer, token string, logBuffer *logs.RingBuffer) *Handler {
	h := &Handler{api: api, mux: http.NewServeMux(), token: token, logBuffer: logBuffer}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for browser console.
	// GET/OPTIONS: allow any origin (read-only, auth is token-based).
	// Mutations (POST/DELETE): echo back the request Origin so only the console
	// (same host) or explicitly-originated requests work; browsers enforce
	// same-origin when no CORS header is present.
	if r.Method == http.MethodOptions || r.Method == http.MethodGet {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Bearer token authentication (skip for OPTIONS already handled above)
	// /metrics is unauthenticated — Prometheus scrapers should not need a token
	if h.token != "" && !strings.HasPrefix(r.URL.Path, "/metrics") {
		auth := r.Header.Get("Authorization")
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
	h.mux.Handle("GET /metrics", promhttp.Handler())
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
	h.mux.HandleFunc("POST /api/v1/validate", h.validateHivefile)
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

// NewServer creates an *http.Server with timeouts, ready for graceful shutdown.
func NewServer(addr string, api hivev1.HiveAPIServer, token string, logBuffer *logs.RingBuffer) *http.Server {
	h := New(api, token, logBuffer)
	return &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}
