# Multi-stage build for hived (Go daemon)
# Usage: docker build -t hived . && docker run -v /var/run/docker.sock:/var/run/docker.sock hived
# SECURITY: Mounting the Docker socket grants equivalent-to-root host access.
# Only run hived in trusted environments. See https://docs.docker.com/engine/security/

# ── Build stage ──────────────────────────────────────────────
FROM golang:1.24-bookworm AS builder

WORKDIR /src
COPY daemon/go.mod daemon/go.sum ./daemon/
RUN cd daemon && go mod download

COPY daemon/ ./daemon/
COPY proto/ ./proto/

RUN cd daemon && CGO_ENABLED=0 go build -ldflags="-s -w" -o /hived ./cmd/hived

# ── Runtime stage ────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /hived /usr/local/bin/hived

EXPOSE 7947 7948 7949 7946/udp

VOLUME ["/var/lib/hive"]

ENTRYPOINT ["hived"]
CMD ["--data-dir", "/var/lib/hive"]
