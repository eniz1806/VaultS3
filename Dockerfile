# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./internal/dashboard/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /vaults3 ./cmd/vaults3

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && \
    addgroup -g 1000 vaults3 && \
    adduser -D -u 1000 -G vaults3 vaults3 && \
    mkdir -p /data /metadata /etc/vaults3 && \
    chown -R vaults3:vaults3 /data /metadata /etc/vaults3

COPY --from=builder /vaults3 /usr/local/bin/vaults3
COPY configs/vaults3.yaml /etc/vaults3/vaults3.yaml

ENV VAULTS3_ACCESS_KEY=""
ENV VAULTS3_SECRET_KEY=""
ENV VAULTS3_DATA_DIR="/data"
ENV VAULTS3_METADATA_DIR="/metadata"

EXPOSE 9000

USER vaults3

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:9000/health || exit 1

ENTRYPOINT ["vaults3"]
CMD ["-config", "/etc/vaults3/vaults3.yaml"]
