# Stage 1: Frontend build (always native — output is static files)
# node:24-alpine — pinned for supply chain integrity
FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:2bdb65ed1dab192432bc31c95f94155ca5ad7fc1392fb7eb7526ab682fa5bf14 AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY docs/contracts/openapi.yaml ../docs/contracts/openapi.yaml
RUN npm run build

# Stage 2: Go build (always native — cross-compile for target arch)
# golang:1.26.4-alpine — pinned for supply chain integrity
FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS builder

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
FROM gcr.io/distroless/static:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240

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
