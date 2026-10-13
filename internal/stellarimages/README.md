# Stellar 2.0 图片元信息

本包负责 Go 抓取／缓存／分析／HTML 回填。生产只消费嵌入物，CGO 关闭，不启动 Node、Rust、命令行图片工具或动态图片库。调用者负责在所有页面渲染完成后保留同一批 `PageSnapshot`，执行 `Prepare`，再对相同 HTML 执行 `Enrich`，最后发布；自动启用开关和 images CLI 由 composition root 装配。

## 接口

- `Analyze([]byte) (Analysis, error)`：转正后的尺寸与 `hsla-mean-v1` 平均色，64px，不放大。
- `AnalyzeContext(ctx, []byte) (Analysis, error)`：同一管线，嵌入解码执行受调用方取消及每次 15 秒上限约束；`Analyze` 保留背景上下文入口。
- `Collect(html, baseURL) ([]string, error)`：按上游渲染作用域扫描，`data-src` 优先，WHATWG HTTP(S) URL 去重，保留首次出现顺序。
- `Processor.Prepare(ctx, []PageSnapshot, PrepareOptions) (Report, error)`：单页选择、dry-run、显式 refresh；3 workers；32MiB、40M 像素、远程请求15秒；缓存完整先于 backoff；失败保留旧尺寸／平均色，24小时后重试。
- `ReadStore(path) (Store, error)`：缺文件返回空 v1；损坏格式／未知版本报错，不能覆盖原文件。
- `Enrich(html, baseURL, Store, EnrichOptions) (string, error)`：只回填派生 HTML，保留作者尺寸／比例；迁移 image 标签的显式尺寸样式到 image-bg；颜色标记保存原 src 供客户端识别换图。

`Processor.LocalAsset` 收到 **相对于 SiteURL 路径的 escaped route**（例如 `/blog/a%20b.png` 传 `a%20b.png`）。本包只剥离站点路径，不 decode、拼接磁盘路径或请求站点回环 HTTP；调用者唯一一次严格 decode，并仅从明确注册的公开资产读取。找不到本地路由报错，不回退到网络。`Processor.Client` 是显式隔离测试接口；生产留 nil，走 safefetch 的逐跳 DNS/IP 校验。`Now` 仅用于可重复验证。

写入以 `O_EXCL` 获取整个批次的 `.lock`，不得删别人的锁。每图结束后用独占临时文件、fsync、rename 原子保存，JSON key 排序，无 fingerprint。dry-run 不创建目录／锁、不抓取、不写入。无内容、原图、数据库副作用。

## 已实现与仍在实现

JPEG／PNG／WebP／AVIF／GIF／TIFF 的栅格管线已具有严格 Sharp 0.34.5 黄金夹具断言，覆盖 EXIF、alpha、8/16位、P3 ICC、CMYK、Lab／scRGB 浮点、JPEG/WebP 解码期缩小、box gap＋固定点 Lanczos3、LZW／Deflate／JPEG／WebP 压缩及 tiled TIFF。AVIF 使用 gav1d 0.2.5；其余嵌入 LittleCMS 2.17、mozjpeg 0826579、libjpeg-turbo 3.1.2、libwebp 1.6.0、libtiff 4.7.1。无系统库回退。

**SVG 完整等效仍在实现中。** 当前过渡嵌入 resvg 0.48.1，含 Noto Sans／Noto Sans SC、72DPI，无宿主文件／网络权限。复杂 SVG 的 tiny-skia 抗锯齿／blur 与上游 librsvg/cairo 有数值差异，完整 oracle 会失败。父任务已要求继续嵌入官方 librsvg/cairo/Pango/GLib，不能用容差替代。此提交只提供可审查的栅格与 Go 生命周期基础，不证明 T6 全部完成。

## 验证与重建

```sh
go test -race ./internal/stellarimages
go run ./internal/stellarimages/oracle
python3 internal/stellarimages/native/build.py --cache .tmp/stellarimages-build --wasi-sdk /path/to/wasi-sdk-29
python3 internal/stellarimages/native/build.py --cache .tmp/stellarimages-build --wasi-sdk /path/to/wasi-sdk-29 --check
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go test -c ./internal/stellarimages -o .tmp/stellarimages-linux-amd64.test
```

完整 oracle 保留 `complex.svg` 的严格四位黄金结果，失败真实公开；栅格单测只断言已闭合的栅格范围。黄金数据来自锁定 Stellar `9d11978` 的 `scripts/lib/image-metadata.js` 和 Sharp 0.34.5。开发期 oracle／重建需要外部工具，生产不需要。重建依赖归档 URL/SHA256 见 `native/build-sources.json`，Rust依赖由 `Cargo.lock` 锁定；许可证及字体授权随嵌入物保留。
