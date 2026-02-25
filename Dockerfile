# Stage 1: Build
FROM golang:1.25-alpine AS builder

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

# Build arguments injected at build time.
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Compile a statically linked binary with version metadata.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -o /bin/documcp ./cmd/server

# Stage 2: Runtime
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the compiled binary.
COPY --from=builder /bin/documcp /documcp

# Copy database migrations for goose.
COPY --from=builder /src/migrations/ /migrations/

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/documcp"]
