# Hive

A lightweight, cross-platform container orchestrator for homelabs and small teams.

Deploy containers across Linux and Windows machines from a single CLI, TUI, or web dashboard — with no control plane, no YAML mountains, and no PhD required.

## What Is This?

Hive sits in the gap between Docker Compose (single machine) and Kubernetes (enterprise complexity). If you run 2-20 machines and want to manage containers across all of them without a week of setup, Hive is for you.

**Key features:**

- **No control plane** — every node is equal, state is shared via SWIM gossip protocol
- **Cross-platform** — Linux and Windows nodes in the same cluster
- **Minimal binaries** — one daemon (`hived`), one CLI (`hive`), one TUI (`hivetop`)
- **TOML config** — readable service definitions, not YAML walls
- **Multi-node gossip mesh** — automatic node discovery and failure detection
- **Encrypted secrets** — age-encrypted at-rest secret store, automatic decrypt on deploy
- **Health checks** — HTTP and TCP health monitoring with configurable intervals

**Planned features (not yet implemented):**

- Internal DNS service discovery
- App Store with one-click deploy recipes
- Web console (Svelte embedded dashboard)

## Architecture

```
hive CLI (Rust)          hivetop TUI (Rust)          Hive Console (Svelte)
       \                       |                        /
        --------  gRPC  --------                --
                    |
        +-----------+-----------+
        |           |           |
    hived (Go)  hived (Go)  hived (Go)
    Linux node  Windows node  ARM node
        |           |           |
        +--- SWIM gossip ------+
```

## Quick Start

```bash
# Install hived on each node
curl -fsSL https://get.hive.dev | sh

# Initialize a cluster on your first node
hive init

# Join additional nodes
hive join <first-node-ip>:7946

# Deploy a service
hive deploy postgres.toml

# Check status
hive ps
hive nodes
```

## Hivefile Example

```toml
[service.web]
image = "nginx:alpine"
replicas = 2

  [service.web.health]
  type = "http"
  path = "/"
  port = 80

  [service.web.ports]
  "8080" = "80"

  [service.web.deploy]
  strategy = "rolling"
```

## Project Structure

| Directory  | Language | Purpose                            |
|------------|----------|------------------------------------|
| `daemon/`  | Go       | Node daemon — container management, networking, gossip |
| `cli/`     | Rust     | Command-line interface              |
| `tui/`     | Rust     | Terminal dashboard (Ratatui)        |
| `console/` | Svelte   | Web dashboard (embedded in daemon)  |
| `proto/`   | Protobuf | gRPC service definitions            |
| `recipes/` | TOML     | One-click app store recipes         |

## Building

```bash
# Build everything
make build

# Build individual components
make build-daemon    # Go daemon (Linux + Windows)
make build-cli       # Rust CLI
make build-tui       # Rust TUI

# Run tests
make test

# Lint
make lint
```

## Development Status

**Phase 0** — Foundation (in progress)

- [x] Repository structure
- [x] Protobuf API definitions
- [x] Single-node container management
- [x] CLI with basic commands
- [x] TUI skeleton

## License

MIT
