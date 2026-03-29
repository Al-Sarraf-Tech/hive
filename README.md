# Hive v2.5.1

**Deploy and manage Docker containers across multiple computers from one place.**

Hive is a single control panel that can install, monitor, and scale your apps on any machine in your network. Point it at your machines, hand it a TOML file describing your services, and it handles the rest — pulling images, scheduling replicas, checking health, managing secrets, and keeping everything running.

No Kubernetes. No YAML mountains. No PhD required.

## What Is This?

Hive sits in the gap between Docker Compose (single machine) and Kubernetes (enterprise complexity). If you run 2-20 machines and want to manage containers across all of them without a week of setup, Hive is for you.

**Core design principles:**

- **No control plane** — every node is equal, state is shared via SWIM gossip protocol
- **Cross-platform** — Linux and Windows nodes in the same cluster
- **Minimal binaries** — one daemon (`hived`), one CLI (`hive`), one TUI (`hivetop`), one web console
- **TOML config** — readable service definitions, not YAML walls
- **Security by default** — mTLS between nodes, encrypted secrets at rest
- **Validate before you deploy** — `hive validate` catches errors before anything touches Docker
- **Health timeline** — per-service health history, not just a snapshot
- **Cluster-wide visibility** — see every container across every node from one dashboard

## Architecture

```
hive CLI (Rust)          hivetop TUI (Rust)          Hive Console (Svelte)
       \                       |                        /
        --------  gRPC  --------              HTTP JSON
                    |                            |
        +-----------+-----------+    +-----------+
        |           |           |    |
    hived (Go)  hived (Go)  hived (Go)
    Linux node  Windows node  ARM node
        |           |           |
        +--- SWIM gossip (UDP) -+
        +--- gRPC mesh (mTLS) --+
```

### Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 7946 | UDP | SWIM gossip — cluster membership, failure detection |
| 7947 | gRPC | Client API — CLI, TUI, integrations (optional TLS) |
| 7948 | gRPC | Mesh API — daemon-to-daemon communication (mTLS) |
| 7949 | HTTP | Web console, Prometheus metrics at `/metrics` |
| 39471 | UDP | WireGuard mesh (optional — encrypted overlay network) |

## Quick Start

```bash
# One-shot install (Linux x86_64)
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash

# Install + set up systemd service
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash -s -- --service --token MY_SECRET

# Build from source (requires Go + Rust)
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash -s -- --local

# Or clone and build locally
git clone https://github.com/Al-Sarraf-Tech/hive.git && cd hive
./install.sh --local --service

# First node — interactive setup (installs Docker if needed, starts daemon)
hive setup

# Second node — join with the code shown by the first node
hive setup --join HIVE-AB12-CD34

# Deploy a service
hive deploy postgres.toml
```

Or manually:

```bash
# Start the daemon (requires Docker or Podman running)
hived --data-dir /var/lib/hive --log-level info

# Initialize a cluster on your first node
hive init --name my-cluster

# Join additional nodes (with the short code or full token)
hive join --code HIVE-AB12-CD34 <first-node-ip>
hive join <first-node-ip>:7946 --token <join-token>

# Deploy a service
hive deploy postgres.toml

# Check status
hive ps
hive nodes
hive status
```

## Hivefile Format

Services are defined in TOML files. One file can contain multiple services with dependencies, health checks, secrets, cron jobs, and resource constraints.

```toml
[service.web]
image = "nginx:alpine"
replicas = 3
platform = "linux/amd64"

  [service.web.health]
  type = "http"
  path = "/"
  port = 80
  interval = "30s"
  timeout = "5s"
  retries = 3

  [service.web.ports]
  "8080" = "80"

  [service.web.env]
  APP_ENV = "production"
  DATABASE_URL = "{{ secret:db-url }}"

  [service.web.resources]
  memory = "512M"
  cpus = 1.0

  [service.web.deploy]
  strategy = "rolling"

  [service.web.depends_on]
  services = ["db", "cache"]

  [[service.web.volumes]]
  name = "web-data"
  target = "/data"
  linux = "/mnt/data:/data"
  windows = "D:\\data:/data"

  [[service.web.cron]]
  schedule = "0 2 * * *"
  command = ["cleanup", "--older-than", "7d"]

[service.db]
image = "postgres:16-alpine"
replicas = 1

  [service.db.health]
  type = "tcp"
  port = 5432

  [service.db.env]
  POSTGRES_DB = "app"
  POSTGRES_PASSWORD = "{{ secret:pg-password }}"
```

### Secret Templating

Environment variables support `{{ secret:key-name }}` syntax. Secrets are decrypted from the encrypted vault at deploy time and injected into the container environment. They never touch disk in plaintext.

## CLI Reference

```
hive setup [--join <code>] [--yes]      Interactive first-run wizard
hive init [--name <cluster>]            Initialize a new cluster
hive join <addrs> --token <token>       Join an existing cluster
hive join --code <HIVE-XXXX-XXXX> <ip>  Join with a short code
hive nodes                              List cluster nodes
hive status                             Show cluster health summary
hive deploy <hivefile.toml>             Deploy services from a Hivefile
hive validate <hivefile.toml> [--server] Validate a Hivefile without deploying
hive ps                                 List running services
hive logs <service> [-f] [-n <lines>]   Stream service logs
hive stop <service>                     Stop a service
hive scale <service> <replicas>         Scale service replicas
hive rollback <service>                 Rollback to previous image
hive restart <service>                  Rolling restart all replicas
hive exec <service> <command...>        Execute a command in a container
hive update <svc> [--image] [--replicas] [--env KEY=VALUE]  Update with rolling restart
hive diff <hivefile.toml>               Dry-run deploy preview
hive backup [--output <path>]           Export cluster state
hive restore <file> [--overwrite]       Import cluster state
hive volume [ls|create|rm]              Manage persistent volumes
hive secret set <key> [<value>]         Set a secret (stdin if no value)
hive secret ls                          List secret metadata
hive secret rm <key>                    Delete a secret
hive secret rotate <key> [<value>]      Rotate secret + rolling-restart referencing services
hive cron                               List active cron jobs
hive daemon [install|start|stop|status] Manage hived as a system service
hive top                                Launch the TUI dashboard
hive app ls [--category <cat>]          Browse available apps
hive app search <query>                 Search apps by name/tag
hive app info <id>                      Show app details and config
hive app install <id> [--config K=V]    Install an app from the catalog
hive app installed                      List installed apps
hive registry login <url> [--username]  Store registry credentials
hive registry ls                        List configured registries
hive registry rm <url>                  Remove registry credentials
```

**Global flags:**
- `--addr <host:port>` — hived address (default: `127.0.0.1:7947`, or `$HIVE_ADDR`)
- `--ca-cert <path>` — TLS CA certificate (or `$HIVE_CA_CERT`)

## TUI Dashboard

`hivetop` provides a real-time terminal dashboard with 4 tabs:

| Tab | Key | Content |
|-----|-----|---------|
| Overview | `1` | Cluster summary — total nodes, services, containers |
| Nodes | `2` | Node list — status, resources, uptime |
| Services | `3` | Service list — replicas, status, health |
| Logs | `4` | Real-time event stream |

```bash
hivetop --addr 127.0.0.1:7947 --refresh 2
```

Controls: `1-4` switch tabs, `q`/`Esc` quit.

## Web Console

The Svelte 5 web console connects to the HTTP API on port 7949 and provides 18 pages:

- **Overview** — cluster stats, node list, recent events, containers per node
- **Services** — service list with replicas, status, health badges
- **Service Detail** — 6 tabs: Overview, Containers, Config, Health Timeline, Logs, Exec
- **Nodes** — node list with CPU/memory/disk, drain controls
- **Node Detail** — system info, resource bars, containers on this node
- **Containers** — cluster-wide container list with service/node filters
- **Logs** — live log viewer with service filter, line count, auto-refresh
- **Cron** — scheduled job list with next/last run times
- **Deploy** — TOML editor with templates (nginx, postgres, redis), validate button, deploy
- **Secrets** — add/delete secrets, masked values
- **App Store** — browse 35 curated apps (publicly, no login required), featured section, 14 category filters, LinuxServer.io collection with LSIO badges, one-click install with TOML preview
- **Learn** — interactive TOML tutorial with syntax-highlighted examples, live validation playground, and complete Hivefile reference card
- **Users** — user management with role assignment, create/delete users (admin only)
- **Backup** — export/import cluster state (services, secrets, config) as encrypted backup files
- **Cluster** — web-based cluster init and node join wizard (no CLI required)
- **Settings** — Docker registry credentials, cluster version info
- **Login** — auto-detects auth mode: first-time admin setup, username/password, or legacy bearer token

**Enhanced existing pages:**
- **Services** — inline quick-update modal (change image, replicas, env vars without redeploying TOML)
- **Nodes Detail** — interactive label management (add/remove labels for placement constraints)
- **Secrets** — secret rotation with affected-service preview and auto rolling-restart
- **Dashboard** — live pulse indicator, quick-action buttons (Deploy, App Store)

The console features a polished dark theme with SVG iconography, smooth animations, glassmorphism cards, gradient accents, and responsive mobile layout. It is compiled to static HTML/CSS/JS and served directly by `hived`.

## Daemon Configuration

`hived` reads configuration from a TOML file, with CLI flags taking precedence.

**Default config path:**
- Linux: `~/.config/hive/hived.toml` (or `$XDG_CONFIG_HOME/hive/hived.toml`)
- Windows: `%APPDATA%\Hive\hived.toml`

```toml
[node]
name = "worker-01"
advertise_addr = "192.168.1.100"
data_dir = "/var/lib/hive"
join = "192.168.1.10:7946,192.168.1.11:7946"

[ports]
grpc = 7947
gossip = 7946
mesh = 7948

[security]
tls = true
gossip_key = "hex-encoded-aes256-key"   # 32, 48, or 64 hex chars

[logging]
level = "info"   # debug, info, warn, error

[http]
port = 7949
token = "bearer-token-for-console"      # optional, protects HTTP API

[wireguard]
enabled = true                          # encrypted mesh overlay (optional)
port = 39471                            # UDP port for WireGuard tunnel
```

**CLI flags** (override config file):
```
hived [options]
  -config <path>           Config file path
  -name <nodename>         Node name (default: hostname)
  -grpc-port <port>        gRPC API port (default: 7947)
  -gossip-port <port>      Gossip UDP port (default: 7946)
  -mesh-port <port>        Mesh gRPC port (default: 7948)
  -http-port <port>        HTTP API port (default: 7949, 0 to disable)
  -advertise-addr <addr>   Address advertised to peers
  -join <addrs>            Comma-separated gossip addresses to join
  -data-dir <path>         State directory
  -log-level <level>       Log level
  -gossip-key <hex>        AES-256 gossip encryption key
  -tls <bool>              Enable TLS on gRPC API
  -wg                      Enable WireGuard mesh overlay
  -wg-port <port>          WireGuard UDP port (default: 39471)
```

## Recipes

The `recipes/` directory contains one-click deploy templates:

| Recipe | Image | Description |
|--------|-------|-------------|
| `postgres` | `postgres:16-alpine` | PostgreSQL with TCP health check, persistent volume |
| `redis` | `redis:7-alpine` | Redis with configurable max memory |
| `nginx` | `nginx:alpine` | Nginx with HTTP health check, rolling deploy |

Deploy a recipe: `hive deploy recipes/postgres/recipe.toml`

## Features

### Gossip Mesh
Nodes discover each other and share state via the SWIM protocol (hashicorp/memberlist). Gossip runs on UDP port 7946. Nodes automatically detect failures and update cluster membership. Gossip traffic can be encrypted with a shared AES-256 key.

### mTLS PKI
Hive generates a self-signed ECDSA P-256 Certificate Authority on cluster init. Each node gets a certificate signed by the CA. Node-to-node mesh traffic (port 7948) uses mTLS. Certificates auto-renew via CSR signing through mesh peers.

### Encrypted Secrets
Secrets are encrypted at rest using [age](https://age-encryption.org/) (X25519). They're stored in the local bbolt database and replicated across the cluster via the mesh. On deploy, `{{ secret:key }}` placeholders in environment variables are resolved and injected — plaintext values never touch disk.

### WireGuard Mesh (Optional)
Enable with `-wg` flag or `[wireguard] enabled = true` in config. Each node gets a deterministic `10.47.X.X` mesh address derived from its WireGuard public key. Keys are exchanged automatically via gossip. All daemon-to-daemon gRPC traffic routes through the encrypted tunnel using a userspace TCP/IP stack (no root or kernel modules required). Works across NAT with persistent keepalive.

### Ingress Load Balancer (v1.1+)
Any service can opt into automatic load balancing by adding `[service.X.ingress]`:

```toml
[service.web.ingress]
port = 8080          # external port
tls = true           # optional: HTTPS with auto-generated self-signed cert
```

Hive creates an nginx proxy container (`hive-ingress-{service}`) that distributes traffic across all healthy replicas. When a replica fails health checks, it's removed from the upstream pool. When it recovers, it's added back automatically. Supports TLS termination with custom or self-signed certificates.

### Canary Deploys (v2.0)
```toml
[service.web.deploy]
strategy = "canary"
canary_weight = 10   # 10% of traffic to canary
```

Deploys one canary replica, health checks it, then promotes by rolling-replacing all existing replicas. Safe automatic rollback if the canary fails.

### Secrets Rotation (v2.0)
`hive secret rotate KEY` updates a secret value and automatically rolling-restarts every service that references it via `{{ secret:KEY }}` templates. Zero-touch secret rotation across the cluster.

### Placement Constraints (v2.0)
Label nodes and constrain service placement:
```bash
hive node label add worker-01 gpu=true
```
```toml
[service.ml-model]
constraints = ["gpu=true", "memory>8GB"]
```

### Multi-Hivefile Stacks (v2.0)
Deploy multiple Hivefiles as a single unit with shared networking:
```toml
name = "my-stack"
[[stack]]
file = "frontend.toml"
[[stack]]
file = "backend.toml"
```

### Autoscaling (v2.0)
```toml
[service.web.autoscale]
min = 1
max = 10
cpu_target = 70       # scale up when CPU > 70%
cooldown_up = "60s"
cooldown_down = "300s"
```

### App Store (v2.5.1)
35 built-in apps ready to deploy in one command. The App Store is **publicly browsable without authentication** — sign in only when you're ready to deploy.

**Core Infrastructure:**

| App | Category | Image |
|-----|----------|-------|
| PostgreSQL | database | postgres:16-alpine |
| MySQL | database | mysql:8 |
| MongoDB | database | mongo:7 |
| InfluxDB | database | influxdb:2-alpine |
| Redis | cache | redis:7-alpine |
| Valkey | cache | valkey/valkey:8-alpine |
| Nginx | webserver | nginx:alpine |
| Caddy | webserver | caddy:2-alpine |
| RabbitMQ | messaging | rabbitmq:3-management-alpine |
| Grafana | monitoring | grafana/grafana:11-alpine |
| Prometheus | monitoring | prom/prometheus:latest |
| Loki | monitoring | grafana/loki:3.0.0 |
| Uptime Kuma | monitoring | louislam/uptime-kuma:1 |
| Traefik | proxy | traefik:v3 |
| MinIO | storage | minio/minio:latest |
| Gitea | devtools | gitea/gitea:latest |
| Docker Registry | devtools | registry:2 |
| n8n | automation | n8nio/n8n:latest |
| Keycloak | security | quay.io/keycloak/keycloak:25.0 |
| HashiCorp Vault | security | hashicorp/vault:1.17 |

**LinuxServer.io Collection** (15 apps with PUID/PGID/TZ, `/config` volume pattern):

| App | Category | Image |
|-----|----------|-------|
| Plex | media | lscr.io/linuxserver/plex |
| Jellyfin | media | lscr.io/linuxserver/jellyfin |
| Sonarr | media | lscr.io/linuxserver/sonarr |
| Radarr | media | lscr.io/linuxserver/radarr |
| Prowlarr | media | lscr.io/linuxserver/prowlarr |
| qBittorrent | media | lscr.io/linuxserver/qbittorrent |
| Overseerr | media | lscr.io/linuxserver/overseerr |
| Transmission | media | lscr.io/linuxserver/transmission |
| Nextcloud | productivity | lscr.io/linuxserver/nextcloud |
| Heimdall | productivity | lscr.io/linuxserver/heimdall |
| Syncthing | productivity | lscr.io/linuxserver/syncthing |
| BookStack | productivity | lscr.io/linuxserver/bookstack |
| FreshRSS | productivity | lscr.io/linuxserver/freshrss |
| Code Server | devtools | lscr.io/linuxserver/code-server |
| WireGuard VPN | networking | lscr.io/linuxserver/wireguard |

```bash
hive app ls                                    # browse catalog
hive app ls --category media                   # filter by category
hive app install postgres --config db_password=secret  # one-click deploy
```

Users can also add custom apps via the API or the web console.

### Interactive Tutorial (v2.5.1)
The web console includes a built-in **Learn** tab with:
- Step-by-step TOML syntax guide with syntax-highlighted examples
- Coverage of all Hivefile features: services, health checks, volumes, secrets, deploy strategies, ingress, cron, autoscaling
- **Live playground** — write TOML and validate against the daemon in real-time
- Complete field reference card
- Copy-to-clipboard and deploy-from-playground buttons

### One-Shot Installer (v2.5.1)
Install Hive on any Linux x86_64 machine with a single command:
```bash
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash
```

Options:
- `--local` — build from source instead of downloading binaries
- `--service` — set up `hived` as a systemd service
- `--token TOKEN` — configure HTTP API bearer token
- `--version VER` — install a specific version

### User Authentication (v2.5.1)
Hive now includes a full user authentication system with:
- **argon2id** password hashing (OWASP recommended, memory-hard, GPU-resistant)
- **HMAC-SHA256 JWT** tokens with auto-generated 256-bit signing keys
- **Role-based access**: `admin` (full), `operator` (deploy/manage), `viewer` (read-only)
- **Rate limiting**: 5 failed attempts per 5-minute window per user
- **First-time setup**: on first visit, create an admin account — no config required
- **Backwards compatible**: legacy `--http-token` bearer auth still works alongside user auth

```bash
# First-time setup via CLI
curl -X POST http://localhost:7949/api/v1/auth/setup \
  -d '{"username":"admin","password":"secure-password-here"}'

# Login returns JWT tokens
curl -X POST http://localhost:7949/api/v1/auth/login \
  -d '{"username":"admin","password":"secure-password-here"}'
# → {"access_token":"eyJ...","refresh_token":"eyJ..."}

# User management (admin only)
hive user list
hive user create bob --role operator
hive user delete bob
```

The web console auto-detects whether user auth is configured and presents the appropriate login flow (username/password or legacy bearer token). User data is stored in the same bbolt database as cluster state — no external database required.

### Docker Registry Login (v2.5)
```bash
hive registry login ghcr.io --username myuser  # stores encrypted credentials
hive registry ls                               # list configured registries
```
Credentials are encrypted with age (X25519) and stored in the local vault. When pulling images, hived automatically uses matching registry credentials.

### Admission Webhooks (v2.0)
```toml
[admission]
url = "http://policy-server:8080/admit"
timeout = "10s"
```
Called before every deploy/update/scale — can reject or allow the operation.

### Backup Scheduling (v2.0)
```toml
[backup]
schedule = "0 2 * * *"
path = "/var/lib/hive/backups"
```

### Log Shipping (v2.0)
```toml
[logging]
level = "info"
driver = "file"        # or "syslog"
path = "/var/log/hive/containers.jsonl"
syslog_host = "syslog.internal:514"
```

### Scheduling & Placement
The scheduler evaluates constraints (platform, node pinning, resource requirements, CPU cores, custom labels) and scores nodes by fitness (CPU/memory headroom, container count) to place replicas. Services with `depends_on` are deployed in topological order.

### Scaling & Rollback
`hive scale web 5` distributes replicas across the cluster. `hive rollback web` reverts to the previous image version, preserving all configuration. `hive restart web` performs a rolling restart of all replicas.

### Health Checks
Three check types: HTTP (status code), TCP (port open), and exec (command exit code). Configurable interval, timeout, and retry count. Health status feeds into the scheduler and is visible in CLI, TUI, and web console.

### Cron Jobs
5-field cron expressions (`minute hour day month weekday`) can be embedded in service definitions. The scheduler runs commands inside service containers on schedule.

### Prometheus Metrics
`hived` exposes Prometheus-format metrics on the HTTP API at `/metrics`:
- gRPC request counts per method
- Container counts per node
- System resources (CPU, memory, disk)

### Log Aggregation
`hived` tails logs from all managed containers into a ring buffer (10K entries). Logs are accessible via `hive logs <service> -f` or the `ContainerLogs` streaming RPC.

### Network Isolation
Services can be deployed with `isolation = "strict"` to restrict network access. Managed containers are labeled `hive.managed=true` and `hive.service=<name>` for filtering.

## gRPC API

The API is defined in `proto/hive/v1/` with two services:

### HiveAPI (port 7947) — 30+ RPCs
Client-facing API for CLI, TUI, and web console.

| Category | RPCs |
|----------|------|
| Cluster | `InitCluster`, `JoinCluster`, `GetClusterStatus` |
| Nodes | `ListNodes`, `GetNode`, `DrainNode`, `SetNodeLabel`, `RemoveNodeLabel` |
| Validation | `ValidateHivefile`, `DiffDeploy` |
| Services | `DeployService`, `DeployStack`, `ListServices`, `GetService`, `StopService`, `ScaleService`, `RollbackService`, `RestartService`, `UpdateService` |
| Containers | `ListContainers`, `ContainerLogs` (stream), `ExecContainer` |
| Secrets | `SetSecret`, `ListSecrets`, `DeleteSecret`, `RotateSecret` |
| Events | `StreamEvents` (stream) |
| Cron | `ListCronJobs` |
| Health | `GetServiceHealth` |
| Volumes | `ListVolumes`, `CreateVolume`, `DeleteVolume` |
| Backup | `ExportCluster`, `ImportCluster` |

### HiveMesh (port 7948, mTLS) — 6 RPCs
Internal daemon-to-daemon communication.

| RPC | Purpose |
|-----|---------|
| `SyncState` | Exchange cluster state (nodes, services, containers) |
| `StartContainer` | Request a peer to start a container |
| `StopContainer` | Request a peer to stop a container |
| `PullLogs` | Stream logs from a peer's container |
| `Ping` | Health check with resource reporting |
| `ReplicateSecret` | Distribute encrypted secrets |
| `SignNodeCSR` | Sign a certificate signing request for a new node |

## Project Structure

```
hive/
  daemon/           Go daemon (hived)
    cmd/hived/        Entry point, flag parsing, startup
    internal/
      api/            gRPC server implementation
      config/         TOML config file parsing
      auth/           User auth — argon2id, JWT, RBAC, rate limiting
      container/      Docker/Podman runtime abstraction
      cron/           Cron scheduler (5-field expressions)
      health/         HTTP/TCP/exec health checks
      hivefile/       TOML service definition parser
      httpapi/        HTTP/JSON gateway, Prometheus metrics
      logs/           Ring buffer log aggregation
      mesh/           SWIM gossip (hashicorp/memberlist)
      metrics/        Prometheus metric collectors
      pki/            mTLS certificate authority
      platform/       OS/arch detection
      scheduler/      Replica placement algorithm
      secrets/        age-encrypted secret vault
      store/          bbolt persistent key-value store
      sysinfo/        System resource queries
      joincode/       Short join code encoding (HIVE-XXXX-XXXX)
      wgmesh/         WireGuard mesh overlay (userspace netstack)
      proxy/          Ingress load balancer (nginx proxy management)
      admission/      Admission webhook client
      autoscale/      Horizontal autoscaler
  cli/              Rust CLI (hive) — 24+ commands via gRPC
  tui/              Rust TUI (hivetop) — 4-tab ratatui dashboard with health column
  console/          Svelte 5 web console — 12 pages with polished dark theme
  install.sh        One-shot installer (GitHub releases or local build)
  proto/            Protobuf definitions (api.proto, mesh.proto, types.proto)
  recipes/          TOML one-click deploy templates
  .github/          CI and release workflows
  Dockerfile        Multi-stage distroless build for hived
  Makefile          Build orchestration
```

## Building from Source

**Prerequisites:** Go 1.26+, Rust 1.85+, protoc 29.3+

```bash
# Build everything
make build

# Individual components
make build-daemon       # hived binary → dist/hived
make build-daemon-all   # Cross-compile: linux/amd64, linux/arm64, windows/amd64
make build-cli          # hive binary → dist/hive
make build-tui          # hivetop binary → dist/hivetop

# Test
make test               # Go + Rust tests

# Lint
make lint               # go vet + cargo fmt + clippy

# Format
make fmt                # gofmt + cargo fmt

# Regenerate protobuf (requires protoc + Go plugins)
make proto
```

## Docker

```bash
# Build the image
docker build -t hived .

# Run (requires Docker socket for container management)
docker run -d \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v hive-data:/var/lib/hive \
  -p 7946:7946/udp \
  -p 7947:7947 \
  -p 7948:7948 \
  -p 7949:7949 \
  hived
```

The Dockerfile produces a minimal distroless image running as nonroot.

**Security note:** Mounting the Docker socket grants equivalent-to-root host access. Only run `hived` in trusted environments.

## Security

- **Node authentication** — mTLS on the mesh ensures only nodes with CA-signed certificates can communicate
- **Gossip encryption** — optional AES-256 encryption for SWIM gossip traffic
- **Secrets at rest** — age encryption (X25519) in bbolt; plaintext never written to disk
- **Certificate renewal** — automatic CSR-based renewal prevents certificate expiry
- **HTTP auth** — optional bearer token protects the web console and HTTP API
- **WireGuard overlay** — optional encrypted mesh with userspace WireGuard (no root required)
- **Key file permissions** — 0600 enforced on private keys with symlink attack checks
- **CORS protection** — mutation endpoints enforce same-origin policy; read-only allows any origin
- **Container labeling** — all managed containers tagged `hive.managed=true` for audit

## CI/CD

CI runs on every push to `main`:
- **Repo Guard** — ownership verification + thermal safety
- **Daemon (Go)** — `go vet`, `go test -race`, `govulncheck`, build
- **CLI (Rust)** — `cargo fmt`, `cargo clippy -D warnings`, `cargo test`, `cargo audit`, release build
- **TUI (Rust)** — same as CLI
- **Proto sync** — regenerates protobuf and diffs to catch stale generated code

Release workflow triggers on `v*` tags:
- Builds `hived` for linux-amd64, windows-amd64
- Builds `hive` and `hivetop` for linux-amd64
- Generates SHA-256 checksums
- Creates a GitHub Release with all artifacts

## Releases

Download pre-built binaries from [GitHub Releases](https://github.com/Al-Sarraf-Tech/hive/releases).

| Binary | Platforms | Description |
|--------|-----------|-------------|
| `hived` | linux-amd64, windows-amd64 | Node daemon |
| `hive` | linux-amd64 | CLI |
| `hivetop` | linux-amd64 | TUI dashboard |

## License

MIT
