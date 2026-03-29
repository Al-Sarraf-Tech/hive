package proxy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/hivefile"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Manager handles the lifecycle of ingress proxy containers.
type Manager struct {
	container container.Provider
	store     *store.Store
	dataDir   string
	nodeName  string
	mesh      *mesh.Mesh // nil in single-node mode
	mu        sync.Mutex
}

// NewManager creates a proxy manager.
func NewManager(c container.Provider, s *store.Store, dataDir, nodeName string, m *mesh.Mesh) *Manager {
	return &Manager{
		container: c,
		store:     s,
		dataDir:   dataDir,
		nodeName:  nodeName,
		mesh:      m,
	}
}

// proxyContainerName returns the container name for a service's ingress proxy.
func proxyContainerName(serviceName string) string {
	return fmt.Sprintf("hive-ingress-%s", serviceName)
}

// EnsureProxy creates or updates the ingress proxy for a service.
// If the service has no ingress config (port=0), this is a no-op.
func (m *Manager) EnsureProxy(ctx context.Context, serviceName string, svcDef hivefile.ServiceDef, networkName string) error {
	if svcDef.Ingress.Port == 0 {
		return nil
	}

	// Check node restriction
	if svcDef.Ingress.Node != "" && svcDef.Ingress.Node != m.nodeName {
		slog.Debug("ingress proxy not for this node", "service", serviceName, "target", svcDef.Ingress.Node, "local", m.nodeName)
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect healthy upstreams
	upstreams := m.collectUpstreams(ctx, serviceName, svcDef)
	slog.Info("ingress proxy upstreams collected", "service", serviceName, "count", len(upstreams))

	// Generate nginx config (TLS or plain)
	listenPort := 80
	var conf []byte
	if svcDef.Ingress.TLS {
		listenPort = 443
		conf = GenerateNginxTLSConf(serviceName, listenPort, upstreams)
	} else {
		conf = GenerateNginxConf(serviceName, listenPort, upstreams)
	}

	// Write config atomically
	confDir := filepath.Join(m.dataDir, "ingress", serviceName)
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return fmt.Errorf("create ingress config dir: %w", err)
	}
	confPath := filepath.Join(confDir, "nginx.conf")
	tmpPath := confPath + ".tmp"
	if err := os.WriteFile(tmpPath, conf, 0o644); err != nil {
		return fmt.Errorf("write ingress config: %w", err)
	}
	if err := os.Rename(tmpPath, confPath); err != nil {
		return fmt.Errorf("rename ingress config: %w", err)
	}

	// Check if proxy container already exists
	existing, _ := m.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.ingress": "true",
		"hive.service": serviceName,
	})

	if len(existing) > 0 && existing[0].Status == "running" {
		// Reload nginx config
		_, err := m.container.Exec(ctx, existing[0].ID, []string{"nginx", "-s", "reload"})
		if err != nil {
			slog.Warn("ingress nginx reload failed, recreating", "service", serviceName, "error", err)
			m.container.Stop(ctx, existing[0].ID, 5)
			m.container.Remove(ctx, existing[0].ID)
		} else {
			slog.Info("ingress proxy reloaded", "service", serviceName, "upstreams", len(upstreams))
			return nil
		}
	} else if len(existing) > 0 {
		// Exists but not running — remove and recreate
		m.container.Stop(ctx, existing[0].ID, 5)
		m.container.Remove(ctx, existing[0].ID)
	}

	// Handle TLS certificates
	volumes := []container.VolumeSpec{{
		Source:   confPath,
		Target:   "/etc/nginx/nginx.conf",
		ReadOnly: true,
	}}
	if svcDef.Ingress.TLS {
		certDir := filepath.Join(confDir, "certs")
		if err := os.MkdirAll(certDir, 0o700); err != nil {
			return fmt.Errorf("create cert dir: %w", err)
		}
		certPath := filepath.Join(certDir, "tls.crt")
		keyPath := filepath.Join(certDir, "tls.key")
		if svcDef.Ingress.TLSCert != "" && svcDef.Ingress.TLSKey != "" {
			// Use provided cert/key — copy to ingress dir
			certData, err := os.ReadFile(svcDef.Ingress.TLSCert)
			if err != nil {
				return fmt.Errorf("read TLS cert: %w", err)
			}
			keyData, err := os.ReadFile(svcDef.Ingress.TLSKey)
			if err != nil {
				return fmt.Errorf("read TLS key: %w", err)
			}
			if err := os.WriteFile(certPath, certData, 0o644); err != nil {
				return fmt.Errorf("write TLS cert: %w", err)
			}
			if err := os.WriteFile(keyPath, keyData, 0o600); err != nil {
				return fmt.Errorf("write TLS key: %w", err)
			}
		} else {
			// Generate self-signed cert if none exists
			if _, err := os.Stat(certPath); os.IsNotExist(err) {
				if genErr := generateSelfSignedCert(certPath, keyPath, serviceName); genErr != nil {
					return fmt.Errorf("generate self-signed cert: %w", genErr)
				}
				slog.Info("ingress: generated self-signed TLS certificate", "service", serviceName)
			}
		}
		volumes = append(volumes, container.VolumeSpec{
			Source:   certDir,
			Target:   "/etc/nginx/certs",
			ReadOnly: true,
		})
	}

	containerPort := strconv.Itoa(listenPort)

	// Create proxy container
	spec := container.ContainerSpec{
		Name:  proxyContainerName(serviceName),
		Image: "nginx:alpine",
		Ports: map[string]string{
			strconv.Itoa(svcDef.Ingress.Port): containerPort,
		},
		Volumes: volumes,
		Labels: map[string]string{
			"hive.managed": "true",
			"hive.ingress": "true",
			"hive.service": serviceName,
		},
		RestartPolicy:  "always",
		NetworkName:    networkName,
		NetworkAliases: []string{serviceName + "-ingress"},
	}

	id, err := m.container.CreateAndStart(ctx, spec)
	if err != nil {
		return fmt.Errorf("create ingress proxy container: %w", err)
	}

	slog.Info("ingress proxy created", "service", serviceName, "container", container.ShortID(id), "port", svcDef.Ingress.Port, "upstreams", len(upstreams))
	return nil
}

// RefreshUpstreams recalculates healthy upstreams and reloads the nginx config.
// Called by the health loop when replica health changes.
func (m *Manager) RefreshUpstreams(ctx context.Context, serviceName string) error {
	// Load service definition from store
	data, err := m.store.Get("services", serviceName)
	if err != nil {
		return fmt.Errorf("load service definition: %w", err)
	}
	if data == nil {
		return nil // service doesn't exist — nothing to refresh
	}

	var svcDef hivefile.ServiceDef
	if err := json.Unmarshal(data, &svcDef); err != nil {
		slog.Warn("ingress: corrupt service definition in store", "service", serviceName, "error", err)
		return nil
	}

	if svcDef.Ingress.Port == 0 {
		return nil // no ingress configured
	}

	// Look up the network name
	networkName := ""
	if netData, err := m.store.Get("meta", "network:"+serviceName); err == nil && netData != nil {
		networkName = string(netData)
	}

	return m.EnsureProxy(ctx, serviceName, svcDef, networkName)
}

// RemoveProxy stops and removes the ingress proxy for a service.
func (m *Manager) RemoveProxy(ctx context.Context, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, _ := m.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.ingress": "true",
		"hive.service": serviceName,
	})

	for _, c := range existing {
		if err := m.container.Stop(ctx, c.ID, 5); err != nil {
			slog.Warn("failed to stop ingress proxy", "service", serviceName, "error", err)
		}
		if err := m.container.Remove(ctx, c.ID); err != nil {
			slog.Warn("failed to remove ingress proxy", "service", serviceName, "error", err)
		}
	}

	// Clean up config dir
	confDir := filepath.Join(m.dataDir, "ingress", serviceName)
	os.RemoveAll(confDir)

	slog.Info("ingress proxy removed", "service", serviceName)
	return nil
}

// collectUpstreams gathers all healthy replica endpoints for a service.
func (m *Manager) collectUpstreams(ctx context.Context, serviceName string, svcDef hivefile.ServiceDef) []Upstream {
	var upstreams []Upstream

	// Determine container port from service definition (use lowest host port for determinism)
	containerPort := "80"
	if len(svcDef.Ports) > 0 {
		var ports []string
		for _, cp := range svcDef.Ports {
			ports = append(ports, cp)
		}
		sort.Strings(ports)
		containerPort = ports[0]
	}

	// Local replicas — use Docker network alias for same-network communication
	localContainers, _ := m.container.ListContainers(ctx, map[string]string{
		"hive.managed": "true",
		"hive.service": serviceName,
	})
	for _, c := range localContainers {
		// Skip the ingress proxy itself
		if c.Labels["hive.ingress"] == "true" {
			continue
		}
		if c.Status != "running" {
			continue
		}
		// Use container name as Docker DNS alias on the shared network
		upstreams = append(upstreams, Upstream{
			Addr: fmt.Sprintf("%s:%s", c.Name, containerPort),
		})
	}

	// Remote replicas — query each peer via SyncState
	if m.mesh != nil {
		peers := m.mesh.Peers()
		for _, peer := range peers {
			func() {
				peerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				state, err := peer.MeshClient().SyncState(peerCtx, &emptypb.Empty{})
				if err != nil {
					slog.Debug("ingress: failed to query peer", "peer", peer.Info.Name, "error", err)
					return
				}

				for _, rc := range state.Containers {
					if rc.ServiceName != serviceName {
						continue
					}
					if rc.Status != hivev1.ContainerStatus_CONTAINER_STATUS_RUNNING {
						continue
					}
					// Use peer's advertise address + host port
					for hostPort := range rc.Ports {
						upstreams = append(upstreams, Upstream{
							Addr: fmt.Sprintf("%s:%s", peer.Info.AdvertiseAddr, hostPort),
						})
						break // one port per container is enough
					}
				}
			}()
		}
	}

	return upstreams
}

// generateSelfSignedCert creates a self-signed TLS certificate for the ingress proxy.
func generateSelfSignedCert(certPath, keyPath, serviceName string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   serviceName + ".hive.local",
			Organization: []string{"Hive Ingress"},
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{serviceName, serviceName + ".hive.local", "localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	return nil
}
