# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
# The build stage runs on the builder's native platform and cross-compiles
# (pure Go, CGO off), so multi-arch images need no QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

# Dependencies first: this layer is cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
# Static, stripped, reproducible binaries. seed and simulator ship in the
# image so `docker compose exec backend /app/seed` works without Go on the host.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    export CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH && \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server && \
    go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed && \
    go build -trimpath -ldflags="-s -w" -o /out/simulator ./cmd/simulator

# ---- runtime ----------------------------------------------------------------
# distroless/static:nonroot — no shell, no package manager, runs as UID 65532,
# includes CA certs and tzdata. There is no wget/curl, so the container
# healthcheck runs `/app/server healthcheck` (a built-in HTTP probe of /health)
# instead of pulling in alpine just for a probe tool.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
HEALTHCHECK --interval=10s --timeout=4s --start-period=10s --retries=3 CMD ["/app/server", "healthcheck"]
ENTRYPOINT ["/app/server"]
