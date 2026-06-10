# syntax=docker/dockerfile:1.6
#
# Multi-stage Dockerfile for safephone-backend.
#   - `dev`   : `go run ./cmd/server` against bind-mounted source. Modules and
#               build artefacts cached on named volumes so a `docker compose
#               up --build` doesn't redownload the world.
#   - `build` : compiles the static binary.
#   - `prod`  : tiny scratch-style runtime image (alpine for TLS certs).
#
# docker-compose at repo root targets `dev`. For prod (Railway typically
# builds from source), target `prod` if you self-host.

FROM golang:1.25-alpine AS base
WORKDIR /app
RUN apk add --no-cache git ca-certificates

# ── dev ────────────────────────────────────────────────────────────────────
FROM base AS dev
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
EXPOSE 8080
# `go run` recompiles on each launch — fine for dev since the compose volume
# mount keeps the source live and a container restart picks up edits. If
# you want true hot-reload, swap to `air` (https://github.com/cosmtrek/air).
CMD ["go", "run", "./cmd/server"]

# ── build ──────────────────────────────────────────────────────────────────
FROM base AS build
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags='-s -w' -o /out/server ./cmd/server

# ── prod ───────────────────────────────────────────────────────────────────
FROM alpine:3.20 AS prod
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1001 app
COPY --from=build /out/server /app/server
# Migrations ship with the image so Railway / scripts can run `migrate-up`
# from inside the container if needed.
COPY --from=build /app/migrations /app/migrations
USER app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
