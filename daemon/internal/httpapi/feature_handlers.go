package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
)

// ─── Node: Get + Labels ─────────────────────────────

func (h *Handler) getNode(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resp, err := h.api.GetNode(r.Context(), &hivev1.GetNodeRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) setNodeLabel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	_, err := h.api.SetNodeLabel(r.Context(), &hivev1.SetNodeLabelRequest{
		Node:  name,
		Key:   body.Key,
		Value: body.Value,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "label set"})
}

func (h *Handler) removeNodeLabel(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	key := r.PathValue("key")
	_, err := h.api.RemoveNodeLabel(r.Context(), &hivev1.RemoveNodeLabelRequest{
		Node: name,
		Key:  key,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "label removed"})
}

// ─── Secret Rotation ────────────────────────────────

func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := h.api.RotateSecret(r.Context(), &hivev1.RotateSecretRequest{
		Key:      key,
		NewValue: []byte(body.Value),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

// ─── Cluster Init / Join ────────────────────────────

func (h *Handler) initCluster(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := h.api.InitCluster(r.Context(), &hivev1.InitClusterRequest{
		ClusterName: body.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

func (h *Handler) joinCluster(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Addresses []string `json:"addresses"`
		Token     string   `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := h.api.JoinCluster(r.Context(), &hivev1.JoinClusterRequest{
		SeedAddrs: body.Addresses,
		JoinToken: body.Token,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

// ─── Deploy Stack ───────────────────────────────────

func (h *Handler) deployStack(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name  string   `json:"name"`
		Files []string `json:"files"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 10<<20)).Decode(&body); err != nil {
		jsonError(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := h.api.DeployStack(r.Context(), &hivev1.DeployStackRequest{
		StackName:     body.Name,
		HivefileTomls: body.Files,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}

// ─── Get Service (single) ───────────────────────────

func (h *Handler) getService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	resp, err := h.api.GetService(r.Context(), &hivev1.GetServiceRequest{Name: name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeProto(w, resp)
}
