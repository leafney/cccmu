# Multi-stage build for cccmu application
# ACM Claude积分监控系统 Docker构建文件

# Stage 1: Build frontend
FROM oven/bun:latest AS frontend-builder

WORKDIR /app/web

# Copy frontend source
COPY web/package.json web/bun.lock ./
RUN bun install

# Copy frontend files and build
COPY web/ .
RUN bun run build

# Stage 2: Build backend
FROM golang:1.23.9-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source
COPY server/ ./server/

# Copy built frontend files to embed
COPY --from=frontend-builder /app/web/dist ./server/web/dist

# Build arguments for version information
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown

# Build backend with embedded frontend
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.GitCommit=${GIT_COMMIT}' -X 'main.BuildTime=${BUILD_TIME}'" \
    -o cccmu ./server/main.go

# Stage 3: Final runtime image
FROM alpine:latest

# Install runtime依赖及权限工具
RUN apk --no-cache add ca-certificates tzdata wget shadow su-exec

# Set timezone to Asia/Shanghai
ENV TZ=Asia/Shanghai

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Create app directory
WORKDIR /app

# Copy binary from builder
COPY --from=backend-builder /app/cccmu .

# 复制入口脚本并赋权
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 默认工作目录准备数据目录
RUN mkdir -p /app/data

# Entrypoint 负责权限调整，最终执行主进程
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["./cccmu"]
