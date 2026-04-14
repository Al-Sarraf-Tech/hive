# Hive v2.6.0

**Lightweight cross-platform container orchestrator for 2–20 machines.**

[![CI](https://github.com/Al-Sarraf-Tech/hive/actions/workflows/ci.yml/badge.svg)](https://github.com/Al-Sarraf-Tech/hive/actions/workflows/ci.yml)
[![Release](https://github.com/Al-Sarraf-Tech/hive/actions/workflows/release.yml/badge.svg)](https://github.com/Al-Sarraf-Tech/hive/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Hive fills the gap between Docker Compose (single machine) and Kubernetes (enterprise complexity). Point it at your machines, hand it a TOML file describing your services, and it handles the rest — pulling images, distributing replicas, checking health, managing secrets, and keeping everything running.

**Design principles:**

- No control plane — every node is equal, state shared via SWIM gossip
- Cross-platform — Linux and Windows nodes in the same cluster
- Minimal binaries — one daemon (`hived`), one CLI (`hive`), one TUI (`hivetop`), one web console
- TOML config — readable service definitions, no YAML
- Security by default — mTLS between nodes, age-encrypted secrets at rest, argon2id user auth
- Validate before deploy — `hive validate` catches errors before anything touches Docker

---

## Architecture

```
  hive CLI (Rust)       hivetop TUI (Rust)      Hive Console (Svelte 5)
        |                       |                        |
        |        gRPC (7947)    |             HTTP/JSON (7949)
        +------- hived ─────────+────────────────────────+
                   |
          +--------+--------+
          |                 |
       hived             hived
     (Linux node)     (Windows node)
          |                 |
          +── SWIM gossip (UDP 7946) ──+
          +── gRPC mesh, mTLS (7948) ──+
          +── WireGuard overlay (UDP 39471, optional) ──+
```

### Ports

| Port  | Protocol | Purpose |
|-------|----------|---------|
| 7946  | UDP      | SWIM gossip — cluster membership and failure detection |
| 7947  | gRPC     | Client API — CLI, TUI, integrations (optional TLS) |
| 7948  | gRPC     | Mesh API — daemon-to-daemon communication (mTLS) |
| 7949  | HTTP/S   | Web console + Prometheus metrics at `/metrics` |
| 39471 | UDP      | WireGuard encrypted overlay (optional) |

---

## Quick Start

**Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash
```

**Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.ps1 | iex
```

**Bootstrap a cluster:**
```bash
# First node — interactive wizard (installs Docker if needed, starts daemon, generates join code)
hive setup

# Additional nodes — join with the code shown by the first node
hive setup --join HIVE-AB12-CD34

# Deploy a service
hive deploy postgres.toml

# Check status
hive ps
hive nodes
hive status
```

Or manually without the wizard:

```bash
# Start the daemon (requires Docker or Podman running)
hived --data-dir /var/lib/hive --log-level info

# Initialize a cluster on your first node
hive init --name my-cluster

# Join additional nodes
hive join <first-node-ip>:7946 --token <join-token>
hive join --code HIVE-AB12-CD34 <first-node-ip>

# Deploy
hive deploy postgres.toml
```

---

## Hivefile Format

Services are defined in TOML files. One file can contain multiple services.

```toml
[service.web]
image = "nginx:alpine"
replicas = 3
platform = "linux/amd64"

  [service.web.health]
  type = "http"       # http | tcp | exec
  path = "/"
  port = 80
  interval = "30s"
  timeout = "5s"
  retries = 3

  [service.web.ports]
  "8080" = "80"       # host_port = container_port

  [service.web.env]
  APP_ENV = "production"
  DATABASE_URL = "{{ secret:db-url }}"   # secret injection

  [service.web.resources]
  memory = "512M"
  cpus = 1.0

  [service.web.deploy]
  strategy = "rolling"   # rolling | canary

  [service.web.ingress]
  port = 8080            # external load-balanced port
  tls = true             # HTTPS termination

  [service.web.depends_on]
  services = ["db", "cache"]

  [[service.web.volumes]]
  name = "web-data"
  target = "/data"
  linux = "/mnt/data:/data"
  windows = "D:\\data:/data"

  [[service.web.cron]]
  schedule = "0 2 * * *"   # 5-field cron expression
  command = ["cleanup", "--older-than", "7d"]

  [service.web.autoscale]
  min = 1
  max = 10
  cpu_target = 70          # scale up when CPU > 70%
  cooldown_up = "60s"
  cooldown_down = "300s"

[service.db]
image = "postgres:16-alpine"
replicas = 1
node = "worker-01"          # pin to specific node (optional)

  [service.db.health]
  type = "tcp"
  port = 5432

  [service.db.env]
  POSTGRES_PASSWORD = "{{ secret:pg-password }}"
```

### Secret Templating

Use `{{ secret:key-name }}` in environment values. Secrets are resolved from the encrypted vault at deploy time and injected directly into the container environment — plaintext never touches disk.

### Deploy Strategies

| Strategy | Behavior |
|----------|----------|
| `rolling` | Replace replicas one at a time; default |
| `canary`  | Deploy one replica, health check it, then promote by replacing all existing replicas; automatic rollback on failure |

### Placement Constraints

```bash
hive node label add worker-01 gpu=true
```
```toml
[service.ml-model]
constraints = ["gpu=true", "memory>8GB"]
```

### Multi-Hivefile Stacks

```toml
name = "my-stack"
[[stack]]
file = "frontend.toml"
[[stack]]
file = "backend.toml"
```

---

## CLI Reference

**Global flags:**
- `--addr <host:port>` — hived address (default: `127.0.0.1:7947`, or `$HIVE_ADDR`)
- `--ca-cert <path>` — TLS CA certificate (or `$HIVE_CA_CERT`)

```
hive setup [--join <code>] [--name <n>] [--yes]    Interactive first-run wizard
hive init [--name <cluster>]                        Initialize a new cluster
hive join <addrs...> [--token <t>] [--code <code>]  Join an existing cluster
hive status                                         Show cluster health summary
hive nodes                                          List cluster nodes
hive ps                                             List running services
hive deploy <hivefile.toml>                         Deploy services from a Hivefile
hive diff <hivefile.toml>                           Dry-run deploy preview
hive validate <hivefile.toml> [--server]            Validate a Hivefile (--server checks cluster state)
hive stop <service>                                 Stop a service
hive scale <service> <replicas>                     Scale service replicas
hive rollback <service>                             Roll back to previous image
hive restart <service>                              Rolling restart all replicas
hive update <svc> [--image <img>] [--replicas <n>] [--env KEY=VALUE]
hive exec <service> <command...>                    Execute a command in a container
hive logs [<service>] [-f] [-n <lines>] [--all]    Stream service logs
hive backup [--output <path>]                       Export cluster state to file
hive restore <file> [--overwrite]                   Import cluster state from file
hive volume ls                                      List persistent volumes
hive volume create <name>                           Create a named volume
hive volume rm <name>                               Delete a volume
hive secret set <key> [<value>]                     Set a secret (stdin if no value)
hive secret ls                                      List secret metadata
hive secret rm <key>                                Delete a secret
hive secret rotate <key> [<value>]                  Rotate secret + rolling-restart all referencing services
hive cron                                           List active cron jobs
hive daemon install|start|stop|status               Manage hived as a system service
hive top                                            Launch the TUI dashboard
hive app ls [--category <cat>]                      Browse app catalog
hive app search <query>                             Search apps by name/tag
hive app info <id>                                  Show app details and config fields
hive app install <id> [--name <n>] [--config K=V]  Install an app from the catalog
hive app installed                                  List installed apps
hive registry login <url> [--username <u>]         Store registry credentials
hive registry ls                                    List configured registries
hive registry rm <url>                              Remove registry credentials
```

---

## TUI Dashboard

`hivetop` provides a real-time terminal dashboard:

```bash
hivetop --addr 127.0.0.1:7947 --refresh 2
```

| Tab | Key | Content |
|-----|-----|---------|
| Overview | `1` | Cluster summary — nodes, services, containers |
| Nodes | `2` | Node list — status, resources, uptime |
| Services | `3` | Service list — replicas, status, health |
| Logs | `4` | Real-time event stream |

Controls: `1`–`4` switch tabs, `q`/`Esc` quit.

---

## Web Console

The Svelte 5 web console connects to the HTTP API on port 7949.

**Pages:**

| Page | Auth Required | Description |
|------|--------------|-------------|
| Overview (Dashboard) | Yes | Cluster stats, Quick Deploy grid, recent events |
| Services | Yes | Service list — replicas, status, health badges, inline quick-update |
| Service Detail | Yes | 6 tabs: Overview, Containers, Config, Health Timeline, Logs, Exec |
| Nodes | Yes | Node list with CPU/memory/disk, drain controls |
| Node Detail | Yes | System info, resource bars, containers, label management |
| Containers | Yes | Cluster-wide container list with service/node filters |
| Logs | Yes | Live log viewer with service filter and auto-refresh |
| Cron | Yes | Scheduled job list with next/last run times |
| Deploy | Yes | TOML editor with templates, validate button, deploy |
| Secrets | Yes | Add/delete secrets, secret rotation with affected-service preview |
| Backup | Yes | Export/import cluster state as backup files |
| Cluster | Yes | Web-based cluster init and node join wizard |
| Users | Yes (admin) | User management with role assignment |
| Settings | Yes | Registry credentials, cluster version info |
| Discover | Yes | List unmanaged Docker containers; adopt into Hive |
| Webhooks | Yes | Configure lifecycle webhook endpoints |
| App Store | No | Browse 35 curated apps, search, filter by category, one-click install |
| Learn | No | Interactive TOML tutorial, live validation playground, field reference |
| Login | No | First-time admin setup or username/password login |

The console is compiled to static HTML/CSS/JS and served directly by `hived`. Dark theme, responsive layout.

---

## Daemon Configuration

`hived` reads a TOML config file. CLI flags override config file values.

**Default config paths:**
- Linux: `~/.config/hive/hived.toml` (respects `$XDG_CONFIG_HOME`)
- Windows: `%APPDATA%\Hive\hived.toml`

```toml
[node]
name = "worker-01"
advertise_addr = "192.168.1.100"
data_dir = "/var/lib/hive"
join = "192.168.1.10:7946,192.168.1.11:7946"   # comma-separated peers

[ports]
grpc = 7947
gossip = 7946
mesh = 7948

[security]
tls = true
gossip_key = "hex-encoded-aes256-key"   # 16, 24, or 32 bytes (32, 48, or 64 hex chars)

[logging]
level = "info"          # debug | info | warn | error
driver = ""             # "file" or "syslog" (empty = stdout only)
path = "/var/log/hive/containers.jsonl"
syslog_host = "syslog.internal:514"

[http]
port = 7949
token = "bearer-token"  # optional legacy bearer auth (use user accounts instead)
tls = false             # enable HTTPS on port 7949 (requires hive init)

[wireguard]
enabled = false
port = 39471

[backup]
schedule = "0 2 * * *"
path = "/var/lib/hive/backups"

[admission]
url = "http://policy-server:8080/admit"
timeout = "10s"

[[hooks]]
type = "pre-deploy"     # pre-deploy | post-deploy | pre-stop | health-fail | *
url = "http://internal:8080/hooks"

[[hooks]]
type = "health-fail"
url = "http://alerts.internal:9000/hive"
```

**CLI flags** (override config):

```
hived
  -config <path>         Config file path
  -name <nodename>       Node name (default: hostname)
  -grpc-port <port>      gRPC API port (default: 7947)
  -gossip-port <port>    Gossip UDP port (default: 7946)
  -mesh-port <port>      Mesh gRPC port (default: 7948)
  -http-port <port>      HTTP API port (default: 7949, 0 to disable)
  -http-tls              Enable HTTPS on HTTP API
  -advertise-addr <addr> Address advertised to peers
  -join <addrs>          Comma-separated gossip addresses to join on startup
  -data-dir <path>       State directory
  -log-level <level>     Log level (debug, info, warn, error)
  -gossip-key <hex>      AES-256 gossip encryption key (hex)
  -tls                   Enable TLS on gRPC API port
  -wg                    Enable WireGuard mesh overlay
  -wg-port <port>        WireGuard UDP port (default: 39471)
```

---

## Features

### SWIM Gossip Mesh
Nodes discover each other and share state via the SWIM protocol ([hashicorp/memberlist](https://github.com/hashicorp/memberlist)). Nodes automatically detect failures and update cluster membership. Gossip traffic can be AES-256 encrypted with a shared key.

### mTLS PKI
`hive init` generates a self-signed ECDSA P-256 CA. Each node gets a certificate signed by the CA. Mesh traffic (port 7948) uses mTLS. Certificates auto-renew via CSR signing through mesh peers — no manual rotation required.

### Encrypted Secrets
Secrets are encrypted at rest with [age](https://age-encryption.org/) (X25519) and stored in the local bbolt database, replicated across the cluster. The `{{ secret:key }}` placeholder is resolved at deploy time; plaintext is never written to disk.

### WireGuard Mesh (Optional)
Enable with `-wg` or `[wireguard] enabled = true`. Each node gets a deterministic `10.47.X.X` address derived from its WireGuard public key. Keys exchange automatically via gossip. Daemon-to-daemon traffic routes through the encrypted tunnel using a userspace TCP/IP stack — no root or kernel modules required.

### Ingress Load Balancer
```toml
[service.web.ingress]
port = 8080
tls = true   # HTTPS with auto-generated self-signed cert
```
Hive creates an nginx proxy container (`hive-ingress-{service}`) that distributes traffic across healthy replicas. Unhealthy replicas are removed from the upstream pool and re-added on recovery.

### User Authentication
- **argon2id** password hashing (OWASP recommended, memory-hard)
- **HMAC-SHA256 JWT** tokens with auto-generated 256-bit signing keys
- **Roles:** `admin` (full), `operator` (deploy/manage), `viewer` (read-only)
- **Rate limiting:** 5 failed attempts per 5-minute window per user
- **First-time setup:** Create an admin account on first visit — no config required
- **Backwards compatible:** legacy `--http-token` bearer auth still works alongside user accounts
- User data stored in the same bbolt database as cluster state — no external database

```bash
# First-time setup
curl -X POST http://localhost:7949/api/v1/auth/setup \
  -d '{"username":"admin","password":"secure-password-here"}'

# Login — returns access_token + refresh_token
curl -X POST http://localhost:7949/api/v1/auth/login \
  -d '{"username":"admin","password":"secure-password-here"}'

# User management via web console (Users page, admin only)
```

### Scheduler and Placement
The scheduler evaluates constraints (platform, node pin, resource requirements, custom labels) and scores nodes by fitness (CPU/memory headroom, container count) to place replicas. Services with `depends_on` deploy in topological order.

### Health Checks

| Type | Trigger |
|------|---------|
| `http` | HTTP GET to `path:port`, checks status code |
| `tcp` | TCP connection to `port` |
| `exec` | Command exit code inside the container |

Configurable `interval`, `timeout`, and `retries`. Health status feeds into the scheduler and ingress upstream pool.

### Autoscaling
```toml
[service.web.autoscale]
min = 1
max = 10
cpu_target = 70       # scale up when CPU > 70%
cooldown_up = "60s"
cooldown_down = "300s"
```

### Cron Jobs
5-field cron expressions (`minute hour day month weekday`) embedded in service definitions. Commands run inside service containers on schedule.

### Admission Webhooks
Called before every deploy/update/scale operation. Return an error to reject the operation.

```toml
[admission]
url = "http://policy-server:8080/admit"
timeout = "10s"
```

### Lifecycle Hooks
Fire HTTP POST payloads on lifecycle events.

```toml
[[hooks]]
type = "*"   # pre-deploy | post-deploy | pre-stop | health-fail | *
url = "http://notifier:8080/events"
```

Payload: `{"type":"...", "service":"...", "node":"...", "message":"...", "time":"..."}`

### Registry Credentials
```bash
hive registry login ghcr.io --username myuser
hive registry ls
```
Credentials encrypted with age (X25519) and stored in the local vault. `hived` automatically uses matching credentials when pulling images.

### Container Discovery
Find Docker containers running outside Hive and bring them under management via the web console (Discover page) or gRPC API. Inspects the running container, generates a Hivefile from its config, and deploys through the standard pipeline.

### Prometheus Metrics
`hived` exposes Prometheus-format metrics at `http://localhost:7949/metrics`:
- gRPC request counts per method
- Container counts per node
- CPU, memory, and disk resources

### Log Aggregation
`hived` tails logs from all managed containers into a 10K-entry ring buffer, accessible via `hive logs <service> -f` or the `ContainerLogs` streaming RPC.

### Backup and Restore
```bash
hive backup --output cluster.json   # export services, secrets, config
hive restore cluster.json           # import to a new or existing cluster
```
Scheduled backups supported via `[backup] schedule` config.

---

## App Store

35 built-in apps deployable in one command. Publicly browsable without authentication.

```bash
hive app ls                                     # browse catalog
hive app ls --category media                    # filter
hive app install postgres --config db_password=secret
```

**Core Infrastructure (20 apps):**

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

**LinuxServer.io Collection (15 apps, PUID/PGID/TZ, `/config` volume pattern):**

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

---

## gRPC API

Defined in `proto/hive/v1/` (`api.proto`, `mesh.proto`, `types.proto`).

### HiveAPI (port 7947) — 48 RPCs

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
| Apps | `ListApps`, `GetApp`, `SearchApps`, `InstallApp`, `ListInstalledApps`, `AddCustomApp`, `RemoveCustomApp` |
| Registry | `RegistryLogin`, `ListRegistries`, `RemoveRegistry` |
| Discovery | `DiscoverContainers`, `AdoptContainer`, `ListDisks` |

### HiveMesh (port 7948, mTLS) — 7 RPCs

| RPC | Purpose |
|-----|---------|
| `SyncState` | Exchange cluster state (nodes, services, containers) |
| `StartContainer` | Request a peer to start a container |
| `StopContainer` | Request a peer to stop a container |
| `PullLogs` | Stream logs from a peer's container |
| `Ping` | Health check with resource reporting |
| `ReplicateSecret` | Distribute encrypted secrets |
| `SignNodeCSR` | Sign a certificate signing request for a new node |

---

## Security

| Area | Mechanism |
|------|-----------|
| Node auth | mTLS — only CA-signed certificates can join the mesh |
| Gossip | Optional AES-256 encryption of SWIM traffic |
| Secrets at rest | age encryption (X25519) in bbolt; plaintext never written to disk |
| Cert renewal | Automatic CSR-based renewal via mesh peers |
| HTTP auth | JWT (argon2id, HMAC-SHA256) + optional legacy bearer token |
| WireGuard overlay | Optional encrypted mesh, userspace (no root required) |
| Key files | Mode 0600 enforced with symlink attack checks |
| CORS | Mutation endpoints enforce same-origin; read-only allows any origin |
| Container labeling | All managed containers tagged `hive.managed=true` for audit |
| Docker socket | Mounting the socket grants equivalent-to-root host access — run `hived` in trusted environments only |

---

## Installation

### One-Shot Installer

**Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash

# Options:
# --local                 Build from source instead of downloading
# --version VER           Install a specific version (default: latest)
# --service               Set up hived as a systemd service
# --token TOKEN           Set HTTP API bearer token in systemd unit
# --install-dir DIR       Install to custom directory (default: /usr/local/bin)
```

**Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.ps1 | iex

# Options: -Version, -Service, -Token
```

### Docker

```bash
docker build -t hived .

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

### Pre-Built Binaries

Download from [GitHub Releases](https://github.com/Al-Sarraf-Tech/hive/releases). All releases include SHA-256 checksums (`checksums.sha256`).

| Binary | Platforms | Description |
|--------|-----------|-------------|
| `hived` | linux-amd64, windows-amd64 | Node daemon |
| `hive` | linux-amd64 | CLI |
| `hivetop` | linux-amd64 | TUI dashboard |

---

## Building from Source

**Prerequisites:** Go 1.26+, Rust 1.85+, protoc 29.3+ (only needed to regenerate protos — generated code is committed)

```bash
# Build all components
make build

# Individual components
make build-daemon        # dist/hived (linux, current arch)
make build-daemon-all    # Cross-compile: linux/amd64, linux/arm64, windows/amd64
make build-cli           # dist/hive
make build-tui           # dist/hivetop
make build-console       # console/build/ (requires Node.js + npm)

# Test
make test                # go test ./... + cargo test (cli + tui)

# Lint
make lint                # go vet + staticcheck + cargo fmt + clippy

# Format
make fmt                 # gofmt + cargo fmt

# Regenerate protobuf Go code
make proto               # requires protoc with go + grpc plugins
```

---

## Project Structure

```
hive/
  daemon/                Go daemon (hived)
    cmd/hived/             Entry point, flag parsing, startup
    internal/
      api/                 gRPC server implementation
      auth/                User auth — argon2id, JWT, RBAC, rate limiting
      config/              TOML config parsing
      container/           Docker/Podman runtime abstraction
      cron/                Cron scheduler (5-field expressions)
      health/              HTTP/TCP/exec health checks + history
      hivefile/            TOML service definition parser
      httpapi/             HTTP/JSON gateway, Prometheus metrics, web console serving
      joincode/            Short join code encoding (HIVE-XXXX-XXXX)
      logs/                Ring buffer log aggregation (10K entries)
      mesh/                SWIM gossip (hashicorp/memberlist)
      metrics/             Prometheus metric collectors
      pki/                 mTLS certificate authority, CSR renewal
      platform/            OS/arch detection, platform-specific paths
      proxy/               Ingress load balancer (nginx proxy management)
      scheduler/           Replica placement and scoring algorithm
      secrets/             age-encrypted secret vault
      store/               bbolt persistent key-value store
      sysinfo/             CPU, memory, disk queries
      wgmesh/              WireGuard overlay (userspace netstack)
      appstore/            App catalog and registry credential manager
      admission/           Admission webhook client
      autoscale/           Horizontal autoscaler
      hooks/               Lifecycle webhook delivery
  cli/                   Rust CLI (hive) — 24 subcommands via gRPC
  tui/                   Rust TUI (hivetop) — 4-tab ratatui dashboard
  console/               Svelte 5 web console — static build, 19 pages
  proto/                 Protobuf definitions (api.proto, mesh.proto, types.proto)
  recipes/               TOML one-click deploy templates
  .github/               CI and release workflows
  Dockerfile             Multi-stage distroless build for hived
  Makefile               Build orchestration
  hive.toml.example      Example Hivefile
  install.sh             One-shot Linux installer
  install.ps1            One-shot Windows installer
```

---

## Recipes

The `recipes/` directory has ready-to-deploy TOML templates:

| Recipe | Image | Notes |
|--------|-------|-------|
| `postgres` | postgres:16-alpine | TCP health check, persistent volume |
| `redis` | redis:7-alpine | Configurable max memory |
| `nginx` | nginx:alpine | HTTP health check, rolling deploy |

```bash
hive deploy recipes/postgres/recipe.toml
```

---

## CI/CD

CI runs on every push to `main` and on pull requests:

| Job | Checks |
|-----|--------|
| Repo Guard | Ownership verification, thermal safety |
| Daemon (Go) | `go vet`, `go test -race`, `govulncheck`, build |
| CLI (Rust) | `cargo fmt`, `cargo clippy -D warnings`, `cargo test`, `cargo audit`, release build |
| TUI (Rust) | Same as CLI |

Release workflow triggers on `v*` tags:
- Builds `hived` for linux-amd64, windows-amd64
- Builds `hive` and `hivetop` for linux-amd64
- Generates SHA-256 checksums
- Creates a GitHub Release with all artifacts

This project is governed by the [Haskell Orchestrator](https://github.com/Al-Sarraf-Tech/Haskell-Orchestrator) — pre-push validation and release management across the Al-Sarraf-Tech organization.

---

## License

Apache License 2.0 — Copyright 2026 Al-Sarraf Technologies LLC
