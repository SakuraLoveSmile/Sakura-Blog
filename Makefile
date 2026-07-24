# Stellar 博客项目任务入口
# 固化构建顺序：先 Console SPA（go:embed 编译期要求 console/dist 非空），再 Go 二进制。

BINARY := blog

.PHONY: all build console go-build test run dev clean docker

all: build

## build: 完整构建（console → go 二进制）
build: console go-build

## console: 构建管理后台 SPA（产出 console/dist 供 go:embed）
console:
	cd console && npm ci && npm run build

## go-build: 编译 Go 二进制（要求 console/dist 已存在）
go-build:
	go build -o $(BINARY) ./cmd/server

## test: 静态检查 + 全部测试
test:
	go vet ./...
	go test ./...

## run: embed 模式构建并运行（首次加 -seed 导入文章）
run: build
	./$(BINARY)

## dev: 开发模式（磁盘读模板/资源/console dist，改样式即时生效）
dev: console go-build
	./$(BINARY) -dev

## clean: 清理构建产物
clean:
	rm -f $(BINARY)
	rm -rf console/dist

## docker: 构建 Docker 镜像（多阶段：node 构建 console → go 编译 embed 二进制）
docker:
	docker build -t blog .
