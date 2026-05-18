# syntax=docker/dockerfile:1

# ---- Stage 1: Build Frontend ----
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- Stage 2: Build Backend ----
FROM golang:1.25-alpine AS backend-builder
WORKDIR /src

# Install git for `go mod`
RUN apk add --no-cache git

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .
# Copy built frontend assets to expected location if needed for embedding (though we serve from disk in this refactor)
# In our main.go, we serve "frontend/dist". So we need to make sure the final image has this structure.

ENV CGO_ENABLED=0
RUN go build -o /out/diary ./cmd/diary

# ---- Stage 3: Final Image ----
FROM alpine:3.22.4

# Create non-root user
RUN adduser -D -H -s /sbin/nologin appuser \
  && mkdir -p /app/data /app/frontend/dist \
  && chown -R appuser:appuser /app

WORKDIR /app

# Copy binary
COPY --from=backend-builder /out/diary ./diary

# Copy frontend assets
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

# Copy sample config
COPY config.sample.toml ./config.sample.toml

# Environment variables
ENV DIARY_CONFIG=/app/data/config.toml

EXPOSE 8080

USER appuser

CMD ["./diary"]
