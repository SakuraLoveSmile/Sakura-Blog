# 多阶段构建：console（node 构建 Vue Console SPA）→ builder（编译 Go 二进制，embed 资源）→ runtime（alpine 精简镜像）。
# assets/templates/icons.yml 与 console dist 均已 go:embed 进二进制，runtime 无需 COPY 资源目录。

# ---- console：构建 Vue Console SPA（产出 dist/ 供 go:embed）----
FROM node:22-alpine AS console

WORKDIR /console

# 先拷依赖清单，利用 Docker 层缓存（package-lock 未变则跳过 npm ci）
COPY console/package.json console/package-lock.json ./
RUN npm ci

# 拷 console 源码并构建（vue-tsc 类型检查 + vite build → dist/）
COPY console/ ./
RUN npm run build

# ---- builder ----
FROM golang:1.25-alpine AS builder

WORKDIR /src

# 先拷依赖清单，利用 Docker 层缓存（go.mod 未变则跳过依赖下载）
COPY go.mod go.sum ./
RUN go mod download

# 拷源码（assets/embed.go + console/embed.go + internal/ + cmd/ + config/ + content/）
COPY . .

# 从 console 阶段拷构建产物：go:embed 编译期要求 console/dist 存在且非空。
# （.dockerignore 已排除本地 dist，此处统一用容器内新鲜构建的产物）
COPY --from=console /console/dist ./console/dist

# 静态编译：CGO_ENABLED=0（modernc.org/sqlite 纯 Go，免 CGO），减体积便于 scratch/alpine。
# -ldflags -s 去符号表，-trimpath 去绝对路径前缀。
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/blog ./cmd/server

# ---- runtime ----
FROM alpine:3.20

# ca-certificates：Gravatar 头像镜像/RSS 外链 HTTPS 需要；tzdata：时间戳本地化。
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 二进制
COPY --from=builder /out/blog /app/blog

# 配置 + 种子文章（DB 运行时生成到挂载卷，不进镜像）
COPY config/ /app/config/
COPY content/ /app/content/

# 数据卷：SQLite + WAL 旁路文件持久化
VOLUME ["/app/data"]

ENV BLOG_LISTEN=":8080" \
    BLOG_DB="/app/data/blog.db" \
    BLOG_ROOT="/app" \
    TZ=Asia/Shanghai

EXPOSE 8080

# 默认不 seed（BLOG_SEED=true 首次启动时开启导入）
ENTRYPOINT ["/app/blog"]
