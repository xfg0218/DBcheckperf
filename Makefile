# dbcheckperf Makefile
# 数据库性能检查工具构建脚本

# 变量定义
BINARY_NAME = dbcheckperf
MAIN_PATH = ./cmd/main.go
OUTPUT_DIR = .
OUTPUT = $(OUTPUT_DIR)/$(BINARY_NAME)

# Go 环境
GO = go
GO_BUILD = $(GO) build
GO_TEST = $(GO) test
GO_CLEAN = $(GO) clean
GO_RUN = $(GO) run

# 编译标志
LDFLAGS = -ldflags "-s -w"
BUILD_FLAGS = -v

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
help:
	@echo "dbcheckperf 构建工具"
	@echo ""
	@echo "用法：make [目标]"
	@echo ""
	@echo "目标:"
	@echo "  build       编译项目 (默认)"
	@echo "  run         运行程序 (需要参数)"
	@echo "  test        运行测试"
	@echo "  clean       清理编译文件"
	@echo "  rebuild     重新编译"
	@echo "  fmt         检查代码格式"
	@echo "  lint        运行代码检查"
	@echo "  cross-compile  交叉编译多平台版本"
	@echo "  help        显示帮助信息"
	@echo ""
	@echo "示例:"
	@echo "  make build"
	@echo "  make run ARGS=\"-h localhost -d /tmp -r ds -v\""
	@echo "  make clean"

# 编译项目
build:
	@echo "==> 编译 $(BINARY_NAME)..."
	$(GO_BUILD) $(BUILD_FLAGS) $(LDFLAGS) -o $(OUTPUT) $(MAIN_PATH)
	@echo "==> 编译完成：$(OUTPUT)"

# 运行程序
run: build
	@echo "==> 运行 $(BINARY_NAME) $(ARGS)"
	./$(BINARY_NAME) $(ARGS)

# 运行测试
test:
	@echo "==> 运行测试..."
	$(GO_TEST) ./...

# 清理
clean:
	@echo "==> 清理编译文件..."
	$(GO_CLEAN)
	rm -f $(OUTPUT)
	@echo "==> 清理完成"

# 重新编译
rebuild: clean build

# 检查代码格式
fmt:
	@echo "==> 检查代码格式..."
	$(GO) fmt ./...

# 代码检查
lint:
	@echo "==> 运行代码检查..."
	$(GO) vet ./...

# 交叉编译 (可选)
cross-compile:
	@echo "==> 交叉编译 Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	@echo "==> 交叉编译 Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 $(MAIN_PATH)
	@echo "==> 交叉编译 Darwin AMD64..."
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	@echo "==> 交叉编译 Darwin ARM64..."
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	@echo "==> 交叉编译完成"

.PHONY: build run test clean rebuild fmt lint cross-compile help
