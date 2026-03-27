// Package httpapi provides a lightweight HTTP/JSON gateway that wraps HiveAPI gRPC calls.
// Designed for the web console — no grpc-gateway dependency required.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Handler wraps a HiveAPI gRPC client to serve HTTP/JSON endpoints.
type Handler struct {
	api hivev1.HiveAPIServer
	mux *http.ServeMux
}

// New creates an HTTP handler that delegates to the given gRPC API server.
func New(api hivev1.HiveAPIServer) *Handler {
	h := &Handler{api: api, mux: http.NewServeMux()}
	h.registerRoutes()
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS headers for browser console
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
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
	h.mux.HandleFunc("POST /api/v1/secrets/{key}", h.setSecret)
	h.mux.HandleFunc("DELETE /api/v1/secrets/{key}", h.deleteSecret)
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
	var body struct {
		HivefileToml string `json:"hivefile_toml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
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
	name := r.PathValue("name")
	var body struct {
		Replicas uint32 `json:"replicas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
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
	name := r.PathValue("name")
	var body struct {
		Command []string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
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

func (h *Handler) setSecret(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
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

// writeProto serializes a protobuf message as JSON using protojson.
func writeProto(w http.ResponseWriter, msg proto.Message) {
	w.Header().Set("Content-Type", "application/json")
	data, err := protojson.Marshal(msg)
	if err != nil {
		http.Error(w, `{"error":"failed to marshal response"}`, http.StatusInternalServerError)
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
	code := http.StatusInternalServerError
	msg := err.Error()

	switch {
	case contains(msg, "not found"):
		code = http.StatusNotFound
	case contains(msg, "InvalidArgument"), contains(msg, "invalid"):
		code = http.StatusBadRequest
	case contains(msg, "Unimplemented"):
		code = http.StatusNotImplemented
	case contains(msg, "PermissionDenied"):
		code = http.StatusForbidden
	case contains(msg, "FailedPrecondition"):
		code = http.StatusPreconditionFailed
	case contains(msg, "AlreadyExists"):
		code = http.StatusConflict
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ListenAndServe starts the HTTP API server.
func ListenAndServe(addr string, api hivev1.HiveAPIServer) error {
	h := New(api)
	server := &http.Server{
		Addr:         addr,
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	slog.Info("http api server listening", "addr", addr)
	return server.ListenAndServe()
}
