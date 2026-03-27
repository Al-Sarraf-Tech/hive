package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jalsarraf0/hive/daemon/internal/api"
	"github.com/jalsarraf0/hive/daemon/internal/container"
	"github.com/jalsarraf0/hive/daemon/internal/health"
	"github.com/jalsarraf0/hive/daemon/internal/mesh"
	"github.com/jalsarraf0/hive/daemon/internal/pki"
	"github.com/jalsarraf0/hive/daemon/internal/platform"
	"github.com/jalsarraf0/hive/daemon/internal/scheduler"
	"github.com/jalsarraf0/hive/daemon/internal/secrets"
	"github.com/jalsarraf0/hive/daemon/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func main() {
	// CLI flags
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
	flag.Parse()

	// Configure logging
	var level slog.Level
	switch *flagLogLevel {
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
	nodeName := *flagNodeName
	if nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "hive-node"
		}
		nodeName = hostname
	}
	grpcPort := *flagGRPCPort
	gossipPort := *flagGossipPort
	dataDir := *flagDataDir
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
	gossipKeyHex := *flagGossipKey
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
		AdvertiseAddr: *flagAdvertiseAddr,
		GRPCPort:      grpcPort,
		MeshPort:      *flagMeshPort,
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

	meshLis, err := net.Listen("tcp", fmt.Sprintf(":%d", *flagMeshPort))
	if err != nil {
		slog.Error("failed to listen on mesh port", "port", *flagMeshPort, "error", err)
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
	if *flagTLS && tlsEnabled {
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
	meshServer := api.NewMeshServer(stateStore, containerProvider, nodeName, dataDir)
	api.RegisterMesh(meshGRPC, meshServer)

	// Auto-join if --join flag provided
	if *flagJoinAddrs != "" {
		addrs := strings.Split(*flagJoinAddrs, ",")
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

	healthLoop := health.NewLoop(healthChecker, containerProvider, stateStore, 30*time.Second, hiveMesh.UpdateContainerCount)
	go healthLoop.Start(ctx)

	// Start certificate renewal checker
	if pki.HasNodeCert(dataDir) {
		renewChecker := pki.NewRenewalChecker(dataDir, nil) // nil = log-only, no auto-renewal yet
		go renewChecker.Start(ctx)
	}

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

	slog.Info("hived listening",
		"node", nodeName,
		"api", fmt.Sprintf(":%d", grpcPort),
		"mesh", fmt.Sprintf(":%d", *flagMeshPort),
		"gossip", fmt.Sprintf(":%d", gossipPort),
		"tls", tlsEnabled,
		"members", hiveMesh.Members(),
	)

	// Start mesh gRPC server in background
	go func() {
		slog.Info("mesh server listening", "port", *flagMeshPort, "tls", tlsEnabled)
		if err := meshGRPC.Serve(meshLis); err != nil {
			if ctx.Err() == nil {
				slog.Error("mesh grpc server failed", "error", err)
			}
		}
	}()

	if err := apiGRPC.Serve(lis); err != nil {
		if ctx.Err() != nil {
			slog.Info("hived stopped gracefully")
		} else {
			slog.Error("grpc server failed", "error", err)
			os.Exit(1)
		}
	}
}
