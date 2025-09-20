# CCCMU 部署指南

## 🚀 快速开始

### 使用 Docker (推荐)

```bash
# 拉取最新镜像
docker pull ghcr.io/leafney/cccmu:latest

# 运行容器（快速体验）
docker run -d \
  --name cccmu \
  -p 8080:8080 \
  ghcr.io/leafney/cccmu:latest

# 运行容器（数据持久化）
docker run -d \
  --name cccmu \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ghcr.io/leafney/cccmu:latest

# 访问应用
open http://localhost:8080
```

### 使用二进制文件

1. 从 [Releases](https://github.com/leafney/cccmu/releases) 页面下载对应平台的二进制文件
2. 解压并运行：

```bash
# Linux/macOS
chmod +x cccmu-linux-amd64
./cccmu-linux-amd64

# Windows
cccmu-windows-amd64.exe
```

## 📋 支持的平台

### Docker 镜像
- `linux/amd64` - x86_64 Linux系统
- `linux/arm64` - ARM64 Linux系统 (如树莓派4、Apple Silicon服务器等)

### 二进制文件

**Windows**
- amd64 (x86_64): `cccmu-windows-amd64.zip`

**macOS**
- amd64 (Intel): `cccmu-darwin-amd64.zip`
- arm64 (Apple Silicon): `cccmu-darwin-arm64.zip`

**Linux**
- amd64 (x86_64): `cccmu-linux-amd64.zip`
- arm64 (ARM64): `cccmu-linux-arm64.zip`

## ⚙️ 配置选项

### 命令行参数

```bash
# 查看帮助
./cccmu --help

# 指定端口运行
./cccmu --port 8080

# 启用详细日志
./cccmu --log

# 查看版本信息
./cccmu --version
```

### 环境变量

- `PORT`: 服务端口 (默认: 8080)

## 🐳 Docker 部署

### Docker Compose 示例

#### 基础配置

```yaml
version: '3.8'

services:
  cccmu:
    image: ghcr.io/leafney/cccmu:latest
    container_name: cccmu
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

#### 带命令行参数的配置

```yaml
version: '3.8'

services:
  cccmu:
    image: ghcr.io/leafney/cccmu:latest
    container_name: cccmu
    # 方法1：使用command覆盖默认命令
    command: ["./cccmu", "--port", "8080", "--log"]
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  # 方法2：使用环境变量（推荐）
  cccmu-env:
    image: ghcr.io/leafney/cccmu:latest
    container_name: cccmu-env
    ports:
      - "9090:9090"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=Asia/Shanghai
      - PORT=9090  # 使用环境变量设置端口
    command: ["./cccmu", "--log"]  # 只启用日志，端口通过环境变量设置
    restart: unless-stopped
```

#### 多服务配置示例

```yaml
version: '3.8'

services:
  # 生产服务（静默模式）
  cccmu-prod:
    image: ghcr.io/leafney/cccmu:latest
    container_name: cccmu-prod
    ports:
      - "8080:8080"
    volumes:
      - ./prod-data:/app/data
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

  # 开发服务（启用详细日志）
  cccmu-dev:
    image: ghcr.io/leafney/cccmu:latest
    container_name: cccmu-dev
    command: ["./cccmu", "--port", "8081", "--log"]
    ports:
      - "8081:8081"
    volumes:
      - ./dev-data:/app/data
    environment:
      - TZ=Asia/Shanghai
    restart: unless-stopped
    profiles:
      - dev  # 使用 docker-compose --profile dev up 启动
```

### 使用持久化存储

```bash
# 创建数据目录
mkdir -p ./cccmu-data

# 运行容器并挂载数据目录（默认模式）
docker run -d \
  --name cccmu \
  -p 8080:8080 \
  -v $(pwd)/cccmu-data:/app/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  ghcr.io/leafney/cccmu:latest

# 运行容器并启用详细日志
docker run -d \
  --name cccmu-debug \
  -p 8080:8080 \
  -v $(pwd)/cccmu-data:/app/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  ghcr.io/leafney/cccmu:latest \
  ./cccmu --log

# 使用自定义端口和日志
docker run -d \
  --name cccmu-custom \
  -p 9090:9090 \
  -v $(pwd)/cccmu-data:/app/data \
  -e TZ=Asia/Shanghai \
  --restart unless-stopped \
  ghcr.io/leafney/cccmu:latest \
  ./cccmu --port 9090 --log
```

### Docker Compose 使用方法

```bash
# 启动基础服务
docker-compose up -d

# 启动带参数的服务
docker-compose up -d cccmu-env

# 启动开发环境服务（需要profile）
docker-compose --profile dev up -d cccmu-dev

# 查看日志
docker-compose logs -f cccmu

# 停止所有服务
docker-compose down
```

## 🔧 高级部署

### 反向代理配置

#### Nginx

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

#### Traefik

```yaml
version: '3.8'

services:
  cccmu:
    image: ghcr.io/leafney/cccmu:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.cccmu.rule=Host(\`your-domain.com\`)"
      - "traefik.http.routers.cccmu.entrypoints=websecure"
      - "traefik.http.routers.cccmu.tls.certresolver=le"
      - "traefik.http.services.cccmu.loadbalancer.server.port=8080"
```

### Kubernetes 部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cccmu
  labels:
    app: cccmu
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cccmu
  template:
    metadata:
      labels:
        app: cccmu
    spec:
      containers:
      - name: cccmu
        image: ghcr.io/leafney/cccmu:latest
        ports:
        - containerPort: 8080
        env:
        - name: TZ
          value: "Asia/Shanghai"
        volumeMounts:
        - name: data
          mountPath: /app/data
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: cccmu-data

---
apiVersion: v1
kind: Service
metadata:
  name: cccmu-service
spec:
  selector:
    app: cccmu
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  type: ClusterIP

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: cccmu-data
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

## 📊 监控和日志

### 健康检查

应用提供健康检查端点：
- `GET /health` - 健康检查接口，返回应用状态和版本信息

### 日志配置

```bash
# 启用详细日志
docker run -d \
  --name cccmu \
  -p 8080:8080 \
  ghcr.io/leafney/cccmu:latest \
  ./cccmu --log

# 查看日志
docker logs -f cccmu
```

## 🔒 安全建议

1. **容器隔离**：Docker 容器提供了良好的隔离性
2. **网络安全**：仅暴露必要的端口
3. **数据备份**：定期备份数据目录
4. **版本更新**：及时更新到最新版本
5. **访问控制**：应用内置访问密钥认证机制

## 🆘 故障排除

### 常见问题

1. **端口占用**
   ```bash
   # 检查端口占用
   netstat -tlnp | grep :8080
   # 或使用其他端口
   ./cccmu --port 8081
   ```

2. **权限问题**
   ```bash
   # 确保二进制文件有执行权限
   chmod +x cccmu-linux-amd64
   ```

3. **数据目录访问**
   ```bash
   # 确保数据目录存在
   mkdir -p ./cccmu-data
   ```

### 日志调试

```bash
# 启用详细日志查看问题
./cccmu --log --port 8080
```

## 📈 性能优化

1. **资源限制**：为容器设置合适的CPU和内存限制
2. **数据存储**：使用SSD存储以提高性能
3. **网络优化**：在生产环境中使用CDN加速静态资源

## 🔄 版本升级

### Docker升级

```bash
# 停止旧容器
docker stop cccmu
docker rm cccmu

# 拉取新版本
docker pull ghcr.io/leafney/cccmu:latest

# 重新启动
docker run -d \
  --name cccmu \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  ghcr.io/leafney/cccmu:latest
```

### 二进制文件升级

1. 停止当前运行的应用
2. 下载新版本二进制文件
3. 替换旧版本文件
4. 重新启动应用