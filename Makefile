# Stellar 博客项目任务入口
# 固化构建顺序：先 Console SPA（go:embed 编译期要求 console/dist 非空），再 Go 二进制。

BINARY := blog

.PHONY: all build console go-build test console-test assets-test merge-check run dev clean docker

all: build

## build: 完整构建（console → go 二进制）
build: console go-build

## console: 构建管理后台 SPA（产出 console/dist 供 go:embed）
console:
	cd console && npm ci && npm run build

## go-build: 编译 Go 二进制（要求 console/dist 已存在）
go-build:
	go build -o $(BINARY) ./cmd/server

## test: 静态检查 + 全部测试（Go vet/test + 前端 vue-tsc 类型检查 + assets/js 与 console 行为面机械校验）
test: console-test assets-test
	go vet ./...
	go test ./...

## console-test: 前端最小机械验证（vue-tsc 类型检查 + node:test 行为测试，需已安装 console/node_modules）
console-test:
	cd console && npm run test

## assets-test: 前端行为面机械校验（node:test 内置运行器，零依赖，见 scripts/test/*.test.mjs）
assets-test:
	node --test scripts/test/*.test.mjs

## merge-check: feature 分支合回 dev 的机械化合并封装（在 feature worktree 根目录执行）
## 强制先跑 make test，任一环节失败即以非零码退出、不执行合并；全绿后才把当前分支合入 dev。
## DEV_WORKTREE 指向 dev worktree 路径，默认兄弟目录 ../dev，可按需覆盖。
DEV_WORKTREE ?= ../dev
merge-check:
	@branch=$$(git branch --show-current); \
	case "$$branch" in \
		dev|main|release|"") echo "merge-check: 当前分支 '$$branch' 不是 feature 分支，中止" >&2; exit 1;; \
	esac; \
	$(MAKE) test || { echo "merge-check: make test 失败，中止合并（dev 未被改动）" >&2; exit 1; }; \
	git -C $(DEV_WORKTREE) merge "$$branch"

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
