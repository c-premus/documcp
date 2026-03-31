# Stage 1: Frontend build
# node:22-alpine — pinned for supply chain integrity
FROM node:22-alpine@sha256:1e8b5d68cac394f76c931b266fe5c224c3fe4cdbc33131e064c83b88235fe77e AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY docs/contracts/openapi.yaml ../docs/contracts/openapi.yaml
RUN npm run build

# Stage 2: Go build
# golang:1.26.1-alpine — pinned for supply chain integrity
FROM golang:1.26.1-alpine@sha256:2389ebfa5b7f43eeafbd6be0c3700cc46690ef842ad962f6c5bd6be49ed82039 AS builder

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

# Compile a statically linked binary with version metadata.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /bin/documcp ./cmd/documcp

# Stage 3: Distroless static runtime — no shell, no package manager, no CVEs.
# gcr.io/distroless/static:nonroot includes CA certificates and runs as UID 65534.
FROM gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39

# Reset working directory — distroless:nonroot defaults to /home/nonroot,
# but our paths (binary, migrations) are at the filesystem root.
WORKDIR /

# Copy the compiled binary.
COPY --from=builder /bin/documcp /documcp

# Copy database migrations for goose.
COPY --from=builder /src/migrations/ /migrations/

EXPOSE 8080

ENTRYPOINT ["/documcp"]
CMD ["serve", "--with-worker"]
