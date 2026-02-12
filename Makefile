.PHONY: build run test clean fmt lint release install

# 版本信息
VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go 参数
GO := go
GOFLAGS := -trimpath
LDFLAGS := -s -w \
	-X 'github.com/user/openclaw-go/internal/gateway.Version=$(VERSION)' \
	-X 'github.com/user/openclaw-go/internal/gateway.Commit=$(COMMIT)' \
	-X 'github.com/user/openclaw-go/internal/gateway.BuildTime=$(BUILD_TIME)'

# 输出目录
BIN_DIR := bin
DIST_DIR := dist

# 主程序
MAIN := ./cmd/openclaw

# 平台
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# 默认目标
all: build

# 构建
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/openclaw $(MAIN)
	@echo "Built: $(BIN_DIR)/openclaw"

# 运行
run: build
	$(BIN_DIR)/openclaw gateway

# 开发模式运行 (带有 race detector)
dev:
	$(GO) run -race $(MAIN) gateway

# 测试
test:
	$(GO) test -v -race ./...

# 测试覆盖率
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# 格式化
fmt:
	$(GO) fmt ./...
	gofumpt -l -w .

# 代码检查
lint:
	golangci-lint run ./...

# 清理
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
	rm -f coverage.out coverage.html

# 安装到 GOPATH/bin
install: build
	cp $(BIN_DIR)/openclaw $(GOPATH)/bin/

# 多平台发布构建
release: clean
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
			-o $(DIST_DIR)/openclaw-$${platform%/*}-$${platform#*/}$$([ "$${platform%/*}" = "windows" ] && echo ".exe" || echo "") \
			$(MAIN); \
		echo "Built: $(DIST_DIR)/openclaw-$${platform%/*}-$${platform#*/}"; \
	done

# 生成协议代码 (从 TypeScript Schema)
generate:
	@echo "TODO: Generate Go types from TypeScript schemas"

# 依赖更新
deps:
	$(GO) mod tidy
	$(GO) mod download

# 安装开发工具
tools:
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 基准测试
bench:
	$(GO) test -bench=. -benchmem ./...

# 快速构建 (无优化，用于开发)
quick:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/openclaw $(MAIN)
	@echo "Quick build: $(BIN_DIR)/openclaw"

# 检查
check: fmt lint test

# Docker 构建
docker:
	docker build -t openclaw-go:$(VERSION) .

# 帮助
help:
	@echo "Usage:"
	@echo "  make build     - Build the binary"
	@echo "  make run       - Build and run the gateway"
	@echo "  make dev       - Run in development mode (with race detector)"
	@echo "  make test      - Run tests"
	@echo "  make coverage  - Generate coverage report"
	@echo "  make fmt       - Format code"
	@echo "  make lint      - Run linter"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make install   - Install to GOPATH/bin"
	@echo "  make release   - Build for all platforms"
	@echo "  make deps      - Update dependencies"
	@echo "  make tools     - Install development tools"
