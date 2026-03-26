.PHONY: all build build-daemon build-daemon-all build-cli build-tui test lint clean proto

# Platforms — no macOS, ever
DAEMON_PLATFORMS = linux/amd64 windows/amd64 linux/arm64

all: proto build

# ─── Protobuf ───────────────────────────────────────────────────
proto:
	@command -v protoc >/dev/null 2>&1 || { echo "ERROR: protoc not installed. Generated code is already committed — skip this target or install protoc."; exit 1; }
	@echo "==> Generating protobuf code..."
	cd daemon && protoc \
		--go_out=internal/api/gen --go_opt=paths=source_relative \
		--go-grpc_out=internal/api/gen --go-grpc_opt=paths=source_relative \
		-I ../proto \
		../proto/hive/v1/*.proto
	@echo "==> Protobuf generation complete"

# ─── Build ──────────────────────────────────────────────────────
build: build-daemon build-cli build-tui

build-daemon:
	@mkdir -p dist
	@echo "==> Building hived..."
	cd daemon && CGO_ENABLED=0 go build -o ../dist/hived ./cmd/hived
	@echo "==> hived built: dist/hived"

build-daemon-all:
	@echo "==> Cross-compiling hived..."
	@mkdir -p dist
	@for platform in $(DAEMON_PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		ext=""; \
		if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		echo "    Building hived-$$os-$$arch$$ext"; \
		(cd daemon && CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -o ../dist/hived-$$os-$$arch$$ext ./cmd/hived) || exit 1; \
	done
	@echo "==> All daemon binaries in dist/"

build-cli:
	@mkdir -p dist
	@echo "==> Building hive CLI..."
	cd cli && cargo build --release
	@cp cli/target/release/hive dist/hive 2>/dev/null || true
	@cp cli/target/release/hive dist/hive-$$(uname -s | tr '[:upper:]' '[:lower:]')-$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') 2>/dev/null || true
	@echo "==> hive CLI built"

build-tui:
	@mkdir -p dist
	@echo "==> Building hivetop..."
	cd tui && cargo build --release
	@cp tui/target/release/hivetop dist/hivetop 2>/dev/null || true
	@cp tui/target/release/hivetop dist/hivetop-$$(uname -s | tr '[:upper:]' '[:lower:]')-$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') 2>/dev/null || true
	@echo "==> hivetop built"

# ─── Test ───────────────────────────────────────────────────────
test: test-daemon test-cli test-tui

test-daemon:
	cd daemon && go test ./...

test-cli:
	cd cli && cargo test

test-tui:
	cd tui && cargo test

# ─── Lint ───────────────────────────────────────────────────────
lint: lint-daemon lint-rust

lint-daemon:
	cd daemon && go vet ./...
	@command -v staticcheck >/dev/null 2>&1 && (cd daemon && staticcheck ./...) || echo "staticcheck not installed, skipping"

lint-rust:
	cd cli && cargo fmt --check
	cd cli && cargo clippy -- -D warnings
	cd tui && cargo fmt --check
	cd tui && cargo clippy -- -D warnings

# ─── Clean ──────────────────────────────────────────────────────
clean:
	rm -rf dist/
	cd cli && cargo clean
	cd tui && cargo clean
	cd daemon && go clean ./...
