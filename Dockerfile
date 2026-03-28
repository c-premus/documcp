# Stage 1: Frontend build
# node:22-alpine — pinned for supply chain integrity
FROM node:22-alpine@sha256:92d51e5f20b7ff58faa5a969af1a1cec6cbec3fbff7e0f523242b9b5c85ad887 AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
COPY docs/contracts/openapi.yaml ../docs/contracts/openapi.yaml
RUN npm run build

# Stage 2: Go build
# golang:1.26.1-alpine — pinned for supply chain integrity
FROM golang:1.26.1-alpine@sha256:d337ecb3075f0ec76d81652b3fa52af47c3eba6c8ba9f93b835752df7ce62946 AS builder

# Install build dependencies.
# - git: required for go mod download with private modules
# - poppler-utils: PDF text extraction (used in tests, available at build time)
RUN apk add --no-cache git poppler-utils

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

# Stage 3: Runtime — Alpine with poppler-utils for PDF extraction.
# alpine:3.21 — pinned for supply chain integrity
FROM alpine:3.21@sha256:22e0ec13c0db6b3e1ba3280e831fc50ba7bffe58e81f31670a64b1afede247bc

# Install runtime dependencies for PDF text extraction and TLS.
RUN apk add --no-cache poppler-utils ca-certificates \
    && addgroup -S nonroot && adduser -S nonroot -G nonroot

# Copy the compiled binary.
COPY --from=builder /bin/documcp /documcp

# Copy database migrations for goose.
COPY --from=builder /src/migrations/ /migrations/

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/documcp"]
CMD ["serve", "--with-worker"]
