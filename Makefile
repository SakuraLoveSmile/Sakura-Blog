# Stellar 博客项目任务入口
# 固化构建顺序：先 Console SPA（go:embed 编译期要求 modules/console/web/dist 非空），再 Go 二进制。

BINARY := blog

.PHONY: all build console go-build test console-test assets-test deploy-test module-check release-secret-check release-secret-history-check console-deps css css-drift-check dist-bootstrap generate merge-check pre-commit-check hooks-install run dev clean

all: build

## build: 完整构建（console → go 二进制）
build: console go-build

## console: 构建管理后台 SPA（产出 modules/console/web/dist 供 go:embed；依赖未变更时跳过 pnpm install）
console: console-deps
	cd modules/console/web && pnpm run build

## go-build: 编译 Go 二进制（要求 modules/console/web/dist 已存在）
go-build:
	go build -o $(BINARY) ./cmd/blog

## test: 静态检查 + 全部测试（Go vet + staticcheck + test + 前端 vue-tsc 类型检查 + assets/js 与 console 行为面机械校验）
## Go 命令统一从根 module 执行。
test: dist-bootstrap console-test assets-test deploy-test module-check
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...
	go test ./...

## release-secret-check: 只扫描 Git 跟踪文件中的常见凭据格式，输出路径和类型，不输出匹配内容。
release-secret-check:
	bash scripts/check-release-secrets.sh

## release-secret-history-check: 扫描所有本地 Git refs 的已知凭据模式，只输出 commit/path/type。
release-secret-history-check:
	bash scripts/check-release-secrets-history.sh

## generate: 从 Go API structs 生成 Console TypeScript 声明。
generate:
	go generate ./modules/console/api

## deploy-test: update.sh 的本地部署/回滚测试，不访问网络或真实 systemd。
deploy-test:
	bash deploy/update_test.sh

## dist-bootstrap: 对齐 CI（.github/workflows/ci.yml go job 的 stub 策略）——
## modules/console/web/embed.go 的 //go:embed all:dist 编译期要求 dist 非空；
## vet/test 不需要真实前端产物，dist 缺失或为空时写入占位文件即可，
## 使全新 worktree 下 make test 给出行为信号而不是 embed 编译环境错误。
## 需要真实产物请用 make build / make console（release 产物由 build job 用真实 dist 验证）。
dist-bootstrap:
	@if [ ! -f modules/console/web/dist/index.html ]; then \
		echo "dist-bootstrap: dist 缺失或无 index.html，写入占位文件（与 CI stub 同语义，仅满足 go:embed 编译）"; \
		mkdir -p modules/console/web/dist && echo '<!doctype html>' > modules/console/web/dist/index.html; \
	else \
		echo "dist-bootstrap: dist 已存在，跳过"; \
	fi

## console-deps: 自举前端 workspace 依赖——依赖缺失或 lockfile 指纹变化时执行 pnpm install。
## pnpm-lock.yaml 是 workspace 唯一锁文件，--frozen-lockfile 保证构建不隐式改锁；
## 安装成功后把指纹写入 node_modules，避免仅凭目录存在误判依赖仍是旧版本。
console-deps:
	@set -e; \
	lock_hash="$$(scripts/console-input-hash.sh --lock)"; \
	if [ ! -d node_modules/.pnpm ] || [ ! -e modules/console/web/node_modules/@blog/api-client ] || [ ! -e modules/console/web/node_modules/@blog/components ] || [ ! -e modules/console/web/node_modules/@blog/editor ] || [ ! -e modules/console/web/node_modules/@blog/plugin-runtime ] || [ ! -e modules/console/web/node_modules/@blog/shared ] || [ ! -f node_modules/.sakura-pnpm-lock.sha256 ] || [ "$$(cat node_modules/.sakura-pnpm-lock.sha256)" != "$$lock_hash" ]; then \
		echo "console-deps: workspace 依赖缺失，执行 pnpm install --frozen-lockfile"; \
		pnpm install --frozen-lockfile; \
		printf '%s\n' "$$lock_hash" > node_modules/.sakura-pnpm-lock.sha256; \
	else \
		echo "console-deps: workspace 依赖已安装，跳过 pnpm install"; \
	fi

## css: 本地/agent 维护入口——从 resource/hexo-theme-stellar 的 styl 源重编 modules/stellar/web/assets/css/main.css。
## 依赖 resource/ 主题源（gitignore，CI 无源不可编译）与 modules/stellar/web/node_modules（stylus 0.62.0），
## 不进 test 门禁；产物 main.css 是 git 跟踪文件，重编后 diff 应无漂移。
css:
	@if [ ! -d modules/stellar/web/node_modules ]; then \
		echo "css: Stellar web 依赖缺失，执行 pnpm install（workspace 锁 stylus 0.62.0）"; \
		pnpm install --frozen-lockfile; \
	fi
	cd modules/stellar/web && pnpm exec node build/stylus/compile.js

## css-drift-check: 本地核对入口——重编主题源并校验当前 main.css 与重编结果一致（无漂移）。
## 定位：仅本地/agent 核对用，不接入 make test 门禁、不强制接入 CI；
## resource/ 为 gitignore 本地主题源，CI 无源不可编译，无源环境跳过核对、不视为失败。
## 用法：在存在 resource/ 源的环境运行本目标；
## 工作区手改 main.css 或产物与源编译结果漂移时，cmp 非零退出。
css-drift-check:
	@if [ ! -d resource/hexo-theme-stellar ]; then \
		echo "css-drift-check: resource/ 主题源缺失（gitignore，CI 无源），跳过本地核对"; \
	else \
		css_snapshot=$$(mktemp); \
		trap 'rm -f "$$css_snapshot"' EXIT INT TERM; \
		cp modules/stellar/web/assets/css/main.css "$$css_snapshot" || exit 1; \
		$(MAKE) css >/dev/null || { \
			echo "css-drift-check: 主题源重编失败" >&2; \
			exit 1; \
		}; \
		cmp -s "$$css_snapshot" modules/stellar/web/assets/css/main.css || { \
			echo "css-drift-check: modules/stellar/web/assets/css/main.css 与主题源重编结果不一致（存在手改或漂移）" >&2; \
			exit 1; \
		}; \
		echo "css-drift-check: modules/stellar/web/assets/css/main.css 与主题源重编结果一致，无漂移"; \
	fi

## console-test: 前端最小机械验证（vue-tsc 类型检查 + node:test 行为测试，依赖由 console-deps 自举）
console-test: console-deps
	cd modules/console/web && pnpm run test

## assets-test: 前端行为面机械校验（node:test 内置运行器，零依赖，见 modules/stellar/tests/*.test.mjs）
assets-test:
	node --test modules/stellar/tests/*.test.mjs

## module-check: 静态约束 Console/Stellar/internal 的依赖方向。
module-check:
	sh scripts/check-module-deps.sh

## lint: staticcheck 静态分析（版本钉死，与 CI 的 Staticcheck 步骤一致）。
lint:
	go run honnef.co/go/tools/cmd/staticcheck@v0.7.0 ./...

## merge-check: feature 分支合回 dev 的机械化合并封装（在 feature worktree 根目录执行）
## 强制先跑 make test，任一环节失败即以非零码退出、不执行合并；全绿后才把当前分支合入 dev。
## 合入成功后会在 dev worktree 再跑一次 make test 复验合并后组合态（防 dev 领先、feature 落后时
## 组合态被破坏）；复验失败输出回退指引并以非零码退出，dev 停在合并后的组合态，按指引回退后
## 先 rebase 最新 dev 修复兼容再重试。保持「先 test 后合并」的语义与退出码契约不变。
## 开头探测 core.hooksPath：未指向 .githooks 直接失败（提交门禁要求机械挂载，见 AGENTS.md「机械挂载」），
## 探测只阻断本目标，不改变 make test 门槛本身。
## DEV_WORKTREE 指向 dev worktree 路径；默认从 Git worktree 列表自动定位 refs/heads/dev，
## 可按需显式覆盖。feature worktree 不依赖固定目录层级。
DEV_WORKTREE ?= $(shell git worktree list --porcelain 2>/dev/null | awk '/^worktree / { p = substr($$0, 10) } /^branch refs\/heads\/dev$$/ { print p; exit }')
merge-check:
	@hooks=$$(git config --get core.hooksPath || true); \
	case "$$hooks" in \
		.githooks|./.githooks) ;; \
		"") echo "merge-check: core.hooksPath 未设置（未执行 make hooks-install），提交门禁要求机械挂载 pre-commit hook，请先执行 make hooks-install 后重试" >&2; exit 1;; \
		*) echo "merge-check: core.hooksPath 指向 '$$hooks'（应为 .githooks），提交门禁要求机械挂载 pre-commit hook，请先执行 make hooks-install 后重试" >&2; exit 1;; \
	esac; \
	dev_worktree="$(DEV_WORKTREE)"; \
	if [ -z "$$dev_worktree" ] || [ ! -d "$$dev_worktree" ] || [ "$$(git -C "$$dev_worktree" branch --show-current 2>/dev/null)" != "dev" ]; then \
		echo "merge-check: 无法定位根目录 dev worktree（当前解析值：$$dev_worktree），请显式传 DEV_WORKTREE=<dev worktree 路径>" >&2; exit 1; \
	fi; \
	branch=$$(git branch --show-current); \
	case "$$branch" in \
		dev|main|release|"") echo "merge-check: 当前分支 '$$branch' 不是 feature 分支，中止" >&2; exit 1;; \
	esac; \
	$(MAKE) test || { echo "merge-check: make test 失败，中止合并（dev 未被改动）" >&2; exit 1; }; \
	git -C $(DEV_WORKTREE) merge "$$branch" || { echo "merge-check: 合入 dev 失败（多为冲突），dev 未被改动，请先 rebase 最新 dev 解决冲突后重试" >&2; exit 1; }; \
	echo "merge-check: 已合入 dev（$(DEV_WORKTREE)），复验合并后组合态"; \
	$(MAKE) -C $(DEV_WORKTREE) test || { \
		echo "merge-check: 合并后组合态 make test 失败，dev 已含本次合并但未通过门禁" >&2; \
		echo "merge-check: 回退指引：git -C $(DEV_WORKTREE) reset --hard ORIG_HEAD（dev 回到合并前状态）" >&2; \
		echo "merge-check: 回退后请在本分支 rebase 最新 dev，修复兼容问题后重试 merge-check" >&2; \
		exit 1; \
	}; \
	echo "merge-check: 合并后组合态复验通过，feature '$$branch' 已合入 dev"

## pre-commit-check: 提交前机械检查（覆盖在 dev 等分支直接工作、不走 merge-check 的路径）
## 强制先跑 make test，任一环节失败即以非零码退出、必须中止提交；全绿后才允许 git commit。
## 开头探测 core.hooksPath：未指向 .githooks 直接失败（提交门禁要求机械挂载，见 AGENTS.md「机械挂载」），
## 探测只阻断本目标，不改变 make test 门槛本身。
pre-commit-check:
	@hooks=$$(git config --get core.hooksPath || true); \
	case "$$hooks" in \
		.githooks|./.githooks) ;; \
		"") echo "pre-commit-check: core.hooksPath 未设置（未执行 make hooks-install），提交门禁要求机械挂载 pre-commit hook，请先执行 make hooks-install 后重试" >&2; exit 1;; \
		*) echo "pre-commit-check: core.hooksPath 指向 '$$hooks'（应为 .githooks），提交门禁要求机械挂载 pre-commit hook，请先执行 make hooks-install 后重试" >&2; exit 1;; \
	esac; \
	$(MAKE) test || { echo "pre-commit-check: make test 失败，中止提交" >&2; exit 1; }; \
	echo "pre-commit-check: 全绿，允许提交"

## hooks-install: 为当前 worktree 一次性挂载 pre-commit hook（机械挂载 pre-commit-check 门槛）
## hook 脚本为仓库 tracked 文件 .githooks/pre-commit，调用同一 make test 门槛并以退出码阻断；
## 通过 core.hooksPath 指向 .githooks，每个 worktree 执行一次即可。
hooks-install:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit
	@echo "hooks-install: 已挂载 pre-commit hook（core.hooksPath=.githooks）"

## run: embed 模式构建并运行（首次加 -seed 导入文章）
run: build
	./$(BINARY)

## dev: 开发模式（磁盘读模板/资源/modules/console/web dist，改样式即时生效）
## 前端增量：按 scripts/console-input-hash.sh 覆盖的完整输入指纹判断，避免只看 console/src 漏判。
dev: console-deps
	@set -e; \
	input_hash="$$(scripts/console-input-hash.sh)"; \
	input_hash_file="modules/console/web/dist/.input-hash"; \
	if [ ! -f modules/console/web/dist/index.html ] || [ ! -f "$$input_hash_file" ] || [ "$$(cat "$$input_hash_file")" != "$$input_hash" ]; then \
		echo "dev: Console 输入已变化或产物缺失，构建前端"; \
		(cd modules/console/web && pnpm run build); \
		input_hash="$$(scripts/console-input-hash.sh)"; \
		printf '%s\n' "$$input_hash" > "$$input_hash_file"; \
	else \
		echo "dev: Console 输入未变化，跳过前端构建"; \
	fi
	go build -o $(BINARY) ./cmd/blog
	./$(BINARY) -dev

## clean: 清理构建产物
clean:
	rm -f $(BINARY)
	rm -rf modules/console/web/dist
