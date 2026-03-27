package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/api"
	hivev1 "github.com/jalsarraf0/hive/daemon/internal/api/gen/hive/v1"
	"github.com/jalsarraf0/hive/daemon/internal/config"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/httpapi"
	"github.com/jalsarraf0/hive/daemon/internal/logs"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/metrics"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
	"github.com/jalsarraf0/hive/daemon/internal/platform"
	"github.com/jalsarraf0/hive/daemon/internal/scheduler"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"github.com/jalsarraf0/hive/daemon/internal/sysinfo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func main() {
	// CLI flags
	flagConfigPath := flag.String("config", "", "Config file path (default: platform-specific hived.toml)")
	flagNodeName := flag.String("name", "", "Node name (default: hostname)")
	flagGRPCPort := flag.Int("grpc-port", 7947, "gRPC API port")
	flagGossipPort := flag.Int("gossip-port", 7946, "SWIM gossip port")
	flagAdvertiseAddr := flag.String("advertise-addr", "", "Address to advertise to peers (auto-detect if empty)")
	flagJoinAddrs := flag.String("join", "", "Comma-separated gossip addresses to join on startup")
	flagDataDir := flag.String("data-dir", "", "Data directory (default: platform-specific)")
	flagLogLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flagGossipKey := flag.String("gossip-key", "", "AES-256 key (hex-encoded) for gossip encryption")
	flagMeshPort := flag.Int("mesh-port", 7948, "gRPC port for daemon-to-daemon mesh (mTLS)")
	flagTLS := flag.Bool("tls", false, "Enable TLS for CLI/TUI connections")
	flagHTTPPort := flag.Int("http-port", 7949, "HTTP API port for web console (0 to disable)")
	flag.Parse()

	// Load config file (missing file = defaults, not an error)
	configPath := *flagConfigPath
	if configPath == "" {
		configPath = config.DefaultPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config %s: %v\n", configPath, err)
		os.Exit(1)
	}

	// Build flag overrides — only flags explicitly set on the CLI override config
	var overrides config.FlagOverrides
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "name":
			overrides.Name = flagNodeName
		case "grpc-port":
			overrides.GRPCPort = flagGRPCPort
		case "gossip-port":
			overrides.GossipPort = flagGossipPort
		case "advertise-addr":
			overrides.AdvertiseAddr = flagAdvertiseAddr
		case "join":
			overrides.Join = flagJoinAddrs
		case "data-dir":
			overrides.DataDir = flagDataDir
		case "log-level":
			overrides.LogLevel = flagLogLevel
		case "gossip-key":
			overrides.GossipKey = flagGossipKey
		case "mesh-port":
			overrides.MeshPort = flagMeshPort
		case "tls":
			overrides.TLS = flagTLS
		case "http-port":
			overrides.HTTPPort = flagHTTPPort
		}
	})
	cfg = cfg.Merge(overrides)

	// Configure logging
	var level slog.Level
	switch cfg.Logging.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Resolve configuration
	nodeName := cfg.Node.Name
	if nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "hive-node"
		}
		nodeName = hostname
	}
	grpcPort := cfg.Ports.GRPC
	gossipPort := cfg.Ports.Gossip
	dataDir := cfg.Node.DataDir
	if dataDir == "" {
		dataDir = platform.DefaultDataDir()
	}

	slog.Info("starting hived",
		"node", nodeName,
		"os", platform.OS(),
		"arch", platform.Arch(),
		"data_dir", dataDir,
		"grpc_port", grpcPort,
		"gossip_port", gossipPort,
	)

	// Initialize data directory
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		slog.Error("failed to create data directory", "path", dataDir, "error", err)
		os.Exit(1)
	}

	// Initialize local state store
	stateStore, err := store.Open(dataDir)
	if err != nil {
		slog.Error("failed to open state store", "error", err)
		os.Exit(1)
	}
	defer stateStore.Close()

	// Initialize container provider
	containerProvider, err := container.NewDockerProvider()
	if err != nil {
		slog.Error("failed to connect to container runtime", "error", err)
		os.Exit(1)
	}
	defer containerProvider.Close()
	slog.Info("container runtime connected",
		"runtime", containerProvider.RuntimeName(),
		"capabilities", containerProvider.DetectCapabilities(),
	)

	// Initialize health checker
	healthChecker := health.NewChecker()

	// Initialize mesh (gossip layer)
	// Parse gossip encryption key if provided
	var gossipKey []byte
	gossipKeyHex := cfg.Security.GossipKey
	if gossipKeyHex == "" {
		gossipKeyHex = os.Getenv("HIVE_GOSSIP_KEY")
	}
	if gossipKeyHex != "" {
		var err error
		gossipKey, err = hex.DecodeString(gossipKeyHex)
		if err != nil {
			slog.Error("invalid gossip key (must be hex-encoded)", "error", err)
			os.Exit(1)
		}
		if len(gossipKey) != 16 && len(gossipKey) != 24 && len(gossipKey) != 32 {
			slog.Error("gossip key must be 16, 24, or 32 bytes (32, 48, or 64 hex chars)")
			os.Exit(1)
		}
		slog.Info("gossip encryption enabled", "key_bytes", len(gossipKey))
	}

	// Detect PKI material — enables mTLS if CA + node certs exist
	tlsEnabled := pki.HasCACert(dataDir) && pki.HasNodeCert(dataDir)
	if tlsEnabled {
		slog.Info("mTLS enabled — PKI material found", "data_dir", dataDir)
	} else {
		slog.Info("mTLS disabled — run 'hive init' to generate cluster certificates")
	}

	meshCfg := mesh.Config{
		NodeName:      nodeName,
		AdvertiseAddr: cfg.Node.AdvertiseAddr,
		GRPCPort:      grpcPort,
		MeshPort:      cfg.Ports.Mesh,
		GossipPort:    gossipPort,
		GossipKey:     gossipKey,
		TLSEnabled:    tlsEnabled,
		DataDir:       dataDir,
	}
	hiveMesh, err := mesh.New(meshCfg, containerProvider.RuntimeName(), containerProvider.DetectCapabilities())
	if err != nil {
		slog.Error("failed to initialize mesh", "error", err)
		os.Exit(1)
	}

	// Initialize scheduler
	sched := scheduler.New(hiveMesh, nodeName)

	// Start gRPC listeners
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		slog.Error("failed to listen on API port", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	meshLis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Ports.Mesh))
	if err != nil {
		slog.Error("failed to listen on mesh port", "port", cfg.Ports.Mesh, "error", err)
		os.Exit(1)
	}

	// Initialize secrets vault (age encryption)
	vault, err := secrets.NewVault(dataDir)
	if err != nil {
		slog.Error("failed to initialize secrets vault", "error", err)
		os.Exit(1)
	}
	slog.Info("secrets vault initialized", "public_key", vault.PublicKey())

	// ─── API server (CLI/TUI connections, optional TLS) ─────
	var apiOpts []grpc.ServerOption
	apiOpts = append(apiOpts, grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		metrics.GRPCRequestsTotal.WithLabelValues(info.FullMethod).Inc()
		return handler(ctx, req)
	}))
	if cfg.Security.TLS && tlsEnabled {
		apiTLSCfg, err := pki.APIServerTLSConfig(dataDir)
		if err != nil {
			slog.Error("failed to load API TLS config", "error", err)
			os.Exit(1)
		}
		apiOpts = append(apiOpts, grpc.Creds(credentials.NewTLS(apiTLSCfg)))
		slog.Info("API server TLS enabled")
	}
	apiGRPC := grpc.NewServer(apiOpts...)
	apiServer := api.NewServer(stateStore, containerProvider, healthChecker, nodeName, hiveMesh, sched, vault, dataDir)
	api.Register(apiGRPC, apiServer)
	reflection.Register(apiGRPC)

	// ─── Mesh server (daemon-to-daemon, mTLS when available) ─
	var meshOpts []grpc.ServerOption
	if tlsEnabled {
		meshTLSCfg, err := pki.MeshServerTLSConfig(dataDir)
		if err != nil {
			slog.Error("failed to load mesh TLS config", "error", err)
			os.Exit(1)
		}
		meshOpts = append(meshOpts, grpc.Creds(credentials.NewTLS(meshTLSCfg)))
	}
	meshGRPC := grpc.NewServer(meshOpts...)
	meshServer := api.NewMeshServer(stateStore, containerProvider, nodeName, dataDir, vault.Decrypt)
	api.RegisterMesh(meshGRPC, meshServer)

	// Auto-join if join address(es) provided
	if cfg.Node.Join != "" {
		addrs := strings.Split(cfg.Node.Join, ",")
		n, err := hiveMesh.Join(addrs)
		if err != nil {
			slog.Error("failed to join cluster", "addrs", addrs, "error", err)
			os.Exit(1)
		}
		slog.Info("joined cluster", "nodes_contacted", n, "total_members", hiveMesh.Members())
	}

	// Start health check loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start log aggregation collector
	logCollector := logs.NewCollector(containerProvider, nodeName)
	go logCollector.Start(ctx)

	healthLoop := health.NewLoop(healthChecker, containerProvider, stateStore, 30*time.Second, hiveMesh.UpdateContainerCount)
	go healthLoop.Start(ctx)

	// System resource metrics (update every 30s alongside health checks)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				memTotal, memAvail := sysinfo.MemInfo()
				metrics.SystemMemoryTotal.Set(float64(memTotal))
				metrics.SystemMemoryAvailable.Set(float64(memAvail))
				diskTotal, diskAvail := sysinfo.DiskInfo(dataDir)
				metrics.SystemDiskTotal.Set(float64(diskTotal))
				metrics.SystemDiskAvailable.Set(float64(diskAvail))
			}
		}
	}()

	// Start certificate renewal checker with automatic CSR-based renewal.
	if pki.HasNodeCert(dataDir) {
		renewFn := func() error {
			local := hiveMesh.LocalNode()

			// Build a CSR signing function that tries self-sign first, then peers
			signCSR := func(csrPEM []byte) (certPEM, caCertPEM []byte, err error) {
				joinToken, _ := stateStore.Get("meta", "join_token")

				// If we hold the CA key, self-sign
				if pki.HasCAKey(dataDir) {
					caKey, caCert, loadErr := pki.LoadCA(dataDir, vault.Decrypt)
					if loadErr == nil {
						signed, signErr := pki.SignCSR(caKey, caCert, csrPEM)
						if signErr == nil {
							caPEM, _ := pki.LoadCACertPEM(dataDir)
							return signed, caPEM, nil
						}
					}
				}

				// Otherwise, iterate peers to find one that can sign
				for _, peer := range hiveMesh.Peers() {
					peerConn, connErr := hiveMesh.PeerByName(peer.Info.Name)
					if connErr != nil {
						continue
					}
					resp, rpcErr := peerConn.MeshClient().SignNodeCSR(context.Background(), &hivev1.SignCSRRequest{
						CsrPem:    csrPEM,
						NodeName:  local.Name,
						JoinToken: string(joinToken),
					})
					if rpcErr != nil {
						slog.Debug("peer cannot sign renewal CSR", "peer", peer.Info.Name, "error", rpcErr)
						continue
					}
					return resp.NodeCertPem, resp.CaCertPem, nil
				}
				return nil, nil, fmt.Errorf("no peer could sign the renewal CSR")
			}

			csrPEM, keyPEM, err := pki.GenerateCSR(local.Name, local.AdvertiseAddr)
			if err != nil {
				return fmt.Errorf("generate CSR: %w", err)
			}
			certPEM, caCertPEM, err := signCSR(csrPEM)
			if err != nil {
				return fmt.Errorf("sign CSR: %w", err)
			}
			if err := pki.SaveNodeCert(dataDir, certPEM, keyPEM); err != nil {
				return fmt.Errorf("save renewed cert: %w", err)
			}
			if len(caCertPEM) > 0 {
				_ = pki.SaveCACert(dataDir, caCertPEM)
			}
			return nil
		}

		renewChecker := pki.NewRenewalChecker(dataDir, renewFn)
		go renewChecker.Start(ctx)
	}

	// httpServer is declared here so the shutdown goroutine can reference it.
	// It is initialized later, after the "hived listening" log line.
	var httpServer *http.Server

	// Graceful shutdown with timeout and second-signal force-quit
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down gracefully", "signal", sig)

		// Second signal = force quit
		go func() {
			<-sigCh
			slog.Warn("received second signal, forcing exit")
			os.Exit(1)
		}()

		cancel()
		healthLoop.Stop()
		_ = hiveMesh.Leave(5 * time.Second)

		// Give in-flight RPCs 10 seconds to finish
		done := make(chan struct{})
		go func() {
			if httpServer != nil {
				_ = httpServer.Shutdown(context.Background())
			}
			apiGRPC.GracefulStop()
			meshGRPC.GracefulStop()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			slog.Warn("graceful shutdown timed out, forcing stop")
			apiGRPC.Stop()
			meshGRPC.Stop()
		}
		_ = hiveMesh.Shutdown()
	}()

	httpAddr := "disabled"
	if cfg.HTTP.Port > 0 {
		httpAddr = fmt.Sprintf(":%d", cfg.HTTP.Port)
	}

	slog.Info("hived listening",
		"node", nodeName,
		"api", fmt.Sprintf(":%d", grpcPort),
		"mesh", fmt.Sprintf(":%d", cfg.Ports.Mesh),
		"http", httpAddr,
		"gossip", fmt.Sprintf(":%d", gossipPort),
		"tls", tlsEnabled,
		"members", hiveMesh.Members(),
	)

	// Start mesh gRPC server in background
	go func() {
		slog.Info("mesh server listening", "port", cfg.Ports.Mesh, "tls", tlsEnabled)
		if err := meshGRPC.Serve(meshLis); err != nil {
			if ctx.Err() == nil {
				slog.Error("mesh grpc server failed", "error", err)
			}
		}
	}()

	// Start HTTP API server for web console
	if cfg.HTTP.Port > 0 {
		addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
		httpServer = httpapi.NewServer(addr, apiServer, "", logCollector.Buffer())
		go func() {
			slog.Info("http api server listening", "addr", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("http api server failed", "error", err)
			}
		}()
	}

	if err := apiGRPC.Serve(lis); err != nil {
		if ctx.Err() != nil {
			slog.Info("hived stopped gracefully")
		} else {
			slog.Error("grpc server failed", "error", err)
			os.Exit(1)
		}
	}
}
