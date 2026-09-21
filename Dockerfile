# Sakura-Blog 全量镜像：node 阶段构建 Console/User Center SPA，go 阶段编译
# go:embed 单二进制，最终 debian-slim 运行。构建输入仅为 Git 跟踪文件，
# 不依赖 gitignored 的 resource/（main.css 等产物已随仓库提交）。
#
# 构建（仓库根目录）：
#   docker build -t blog .
# 运行（只挂运行态子目录，不要把整个安装目录挂到 /app——会遮住镜像内二进制）：
#   docker run -d --name blog -p 8081:8080 \
#     -v /opt/blog/config:/app/config \
#     -v /opt/blog/data:/app/data \
#     -v /opt/blog/content:/app/content \
#     blog

# ---- Console / User Center SPA ----
FROM node:20-bookworm-slim AS web
WORKDIR /src
RUN corepack enable
COPY . .
RUN pnpm install --frozen-lockfile \
    && pnpm --filter console run build

# ---- Go 单二进制（go:embed 编译期要求 dist 与 dist-uc 非空）----
FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY . .
COPY --from=web /src/modules/console/web/dist modules/console/web/dist
COPY --from=web /src/modules/console/web/dist-uc modules/console/web/dist-uc

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=
ARG STELLAR_COMMIT=unknown
ARG FRONTEND_HASH=unknown
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w \
        -X blog/internal/app.Version=${VERSION} \
        -X blog/internal/app.GitCommit=${GIT_COMMIT} \
        -X blog/internal/app.BuildTime=${BUILD_TIME} \
        -X blog/internal/app.StellarCommit=${STELLAR_COMMIT} \
        -X blog/internal/app.FrontendFingerprint=${FRONTEND_HASH}" \
      -o /out/blog ./cmd/blog

# ---- 运行时 ----
FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /app
COPY --from=build /out/blog /usr/local/bin/blog
LABEL org.opencontainers.image.source="https://github.com/SakuraLoveSmile/Sakura-Blog" \
      org.opencontainers.image.description="Sakura-Blog：Console SPA + Stellar SSR 的 go:embed 单二进制"
EXPOSE 8080
WORKDIR /app
ENTRYPOINT ["blog", "-root", "/app"]
