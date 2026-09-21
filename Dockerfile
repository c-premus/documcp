# Stage 1: Frontend build (always native — output is static files)
# node:24-alpine — pinned for supply chain integrity
FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1 AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY docs/contracts/openapi.yaml ../docs/contracts/openapi.yaml
RUN npm run build

# Stage 2: Go build (always native — cross-compile for target arch)
# golang:<ver>-alpine@sha256 — pinned by tag and digest for supply chain integrity.
# This tag IS the shipped stdlib: the official golang image sets GOTOOLCHAIN=local,
# so go.mod's `toolchain` directive is ignored here. A stale tag silently produces
# a stale-stdlib binary with no build error (the bare `go` floor still passes).
# ci.yaml's "Go toolchain agreement" step fails the build if the two disagree.
FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

# Target architecture injected by Buildx (e.g., amd64, arm64).
ARG TARGETARCH

# Install build dependencies.
# - git: required for go mod download with private modules
RUN apk add --no-cache git

WORKDIR /src

# Cache dependency downloads before copying full source.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree.
COPY . .

# Copy the built frontend assets into the embed directory.
COPY --from=frontend /app/web/frontend/dist ./web/frontend/dist

# Build arguments injected at build time.
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Cross-compile a statically linked binary for the target architecture.
# -trimpath strips the build host's filesystem prefix from the binary so two
# builds of the same source tree on different machines produce identical bytes.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /bin/documcp ./cmd/documcp

# Stage 3: Distroless static runtime — no shell, no package manager, no CVEs.
# gcr.io/distroless/static:nonroot includes CA certificates and runs as UID 65534.
FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

# Reset working directory — distroless:nonroot defaults to /home/nonroot,
# but our paths (binary, migrations) are at the filesystem root.
WORKDIR /

# Copy the compiled binary.
COPY --from=builder /bin/documcp /documcp

# Copy database migrations for goose.
COPY --from=builder /src/migrations/ /migrations/

EXPOSE 8080 8443

ENTRYPOINT ["/documcp"]
CMD ["serve", "--with-worker"]
