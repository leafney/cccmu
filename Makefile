# Claude Code积分监控系统 构建脚本

# 变量定义
BINARY_NAME=cccmu
FRONTEND_DIR=web
BACKEND_DIR=server
BUILD_DIR=dist

# 默认目标
.PHONY: help
help:
	@echo "可用的命令:"
	@echo "  dev-frontend    - 启动前端开发服务器"
	@echo "  dev-backend     - 启动后端开发服务器"
	@echo "  dev            - 同时启动前后端开发环境"
	@echo "  build-frontend - 构建前端生产版本"
	@echo "  build-backend  - 编译后端二进制文件"
	@echo "  build          - 完整构建项目"
	@echo "  clean          - 清理构建文件"
	@echo "  test           - 运行测试"
	@echo "  fmt            - 格式化代码"
	@echo "  lint           - 代码检查"

# 开发环境
.PHONY: dev-frontend
dev-frontend:
	@echo "启动前端开发服务器..."
	cd $(FRONTEND_DIR) && bun run dev

.PHONY: dev-backend
dev-backend:
	@echo "启动后端开发服务器..."
	cd $(BACKEND_DIR) && go run main.go

.PHONY: dev
dev:
	@echo "启动完整开发环境..."
	@make -j2 dev-frontend dev-backend

# 构建相关
.PHONY: build-frontend
build-frontend:
	@echo "构建前端生产版本..."
	cd $(FRONTEND_DIR) && bun install && bun run build

.PHONY: build-backend
build-backend: build-frontend
	@echo "编译后端二进制文件..."
	cd $(BACKEND_DIR) && go mod tidy && go build -ldflags="-s -w" -o ../$(BINARY_NAME) main.go

.PHONY: build
build: build-backend
	@echo "构建完成: $(BINARY_NAME)"

# 工具命令
.PHONY: clean
clean:
	@echo "清理构建文件..."
	@rm -rf $(FRONTEND_DIR)/dist
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@echo "清理完成"

.PHONY: test
test:
	@echo "运行Go测试..."
	cd $(BACKEND_DIR) && go test -v ./...
	@echo "运行前端测试..."
	cd $(FRONTEND_DIR) && bun test

.PHONY: fmt
fmt:
	@echo "格式化Go代码..."
	cd $(BACKEND_DIR) && go fmt ./...
	@echo "格式化前端代码..."
	cd $(FRONTEND_DIR) && bun run format

.PHONY: lint
lint:
	@echo "Go代码检查..."
	cd $(BACKEND_DIR) && go vet ./...
	@echo "前端代码检查..."
	cd $(FRONTEND_DIR) && bun run lint

# 安装依赖
.PHONY: install
install:
	@echo "安装Go依赖..."
	cd $(BACKEND_DIR) && go mod tidy
	@echo "安装前端依赖..."
	cd $(FRONTEND_DIR) && bun install