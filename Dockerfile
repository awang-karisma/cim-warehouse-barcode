# syntax=docker/dockerfile:1

# Multi-stage Dockerfile for Warehouse Barcode Server

# Stage 1: Build SolidJS Frontend with pnpm
FROM node:22-alpine AS frontend-builder
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

WORKDIR /web

# Copy package manifests and workspace configuration first for layer caching
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./

# Leverage BuildKit cache mount to cache pnpm store across builds
RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile

# Copy source code and build frontend production bundle
COPY web/ ./
RUN pnpm run build

# Stage 2: Build Go Server
FROM golang:1.27-alpine AS backend-builder
ENV GOTOOLCHAIN=local
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
# Copy frontend build output into embed path
COPY --from=frontend-builder /web/dist ./cmd/barcode-server/dist
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build -tags embed_frontend \
    -ldflags="-s -w \
    -X github.com/awang-karisma/cim-warehouse-barcode/pkg/version.Version=${VERSION} \
    -X github.com/awang-karisma/cim-warehouse-barcode/pkg/version.Commit=${COMMIT} \
    -X github.com/awang-karisma/cim-warehouse-barcode/pkg/version.Date=${DATE}" \
    -o /app/barcode-server ./cmd/barcode-server

# Stage 3: Minimal runtime image
FROM alpine:3.21
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=backend-builder /app/barcode-server /app/barcode-server

ENV PORT=8080 \
    DB_PATH=/data/barcode.db

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/barcode-server"]
