# 嵌入式 librsvg WASI 解码器

正式 SVG 图片元信息由 `../librsvg.wasm` 处理。它使用 librsvg 2.61.2、
GLib 2.86.1、Pango 1.57.0、Cairo 1.18.4、Fontconfig 2.17.1、
Harfbuzz 12.1.0 和 FreeType 2.14.1；不是 resvg 近似替代。
Go 单二进制通过 wazero 实例化 WASI。Node、Sharp、宿主图片库、宿主字体、
宿主文件系统和网络均不是生产依赖。Sharp 只用于锁定开发期黄金结果。

## 边界与平台差异

- 32 MiB 输入、40 MP 原始尺寸、最大 64×64 取样、512 MiB WASM 内存、15 秒实例时限。
- 自然尺寸使用 librsvg 的 CSS 像素尺寸；百分比/缺失尺寸按 libvips 的
  viewBox、单边比例、可见几何顺序解析。固定 DPI 为上游默认 72。
- Noto Sans 与 Noto Sans SC 已嵌入，Fontconfig 只读挂载 `/fonts` 和
  `/etc/fonts/fonts.conf`；不写字体缓存。字体来源声明与 OFL 原文在
  `../svg/FONT-LICENSE`、`../svg/FONT-SC-LICENSE`，文件指纹在 `fonts.json`。
- Pango 的字体初始化、匹配和排序在同一 WASI 实例中同步完成，同一官方函数
  保持算法；不创建宿主线程。析构使用真实单参回调，复制使用严格双参适配。
- `../wasi/renderer-file.c` 仅提供官方 GDummyFile 的 URI 身份与解析。
  外部 SVG 文件读取明确拒绝；内嵌 `data:` 资源由 librsvg 解码。
  不提供外部文件、Unix 桌面、DBus、网络或可写磁盘 GIO backend。
- GLib 标量 libffi 和类型初始化通过 `internal/stellarimages/wasmffi`
  对照真实 WASM 签名调用；没有放宽间接调用检查。每图独立实例在结束时关闭。

## 来源、许可证与替换

`build-sources.json`、`../wasi/build-sources.json` 记录原生上游归档 URL 与 SHA256。
Rust 依赖由 `Cargo.lock` 锁定；三个 WASI Rust 移植的原始 crate 指纹及补丁在
`rust-ports.json`。所有平台差异保存为 `.patch` 或本仓库 C/Rust 源文件。

`native-licenses.json` 和 `dependency-licenses.json` 对应 `licenses/` 内的原文。
后者包括整个锁文件（含仅构建/其他平台依赖），如上游源包不包含独立许可证
文件会明确记录；不伪造其版权声明。每个原始 crate 的完整源包有可核对的
SHA256 和下载地址。GLib/库许可证沿用 `../wasi/GLIB-LICENSE`，其余共用
PCRE2、libffi、zlib、WASI libc 许可证保留在已有 `native/`/`native/wasi/` 中。
FreeType 可按其 FTL 授权使用，Cairo 可按其 LGPL-2.1 或 MPL-1.1 授权使用。

使用者可以取得这些完整原始源包、应用本仓库补丁并重建/替换 WASM；
然后正常重建 Go 应用即可重新链接。发布时应随源码与二进制提供这些许可材料。

## 重建

开发工具链：WASI SDK 29（Clang 21.1.4）、Rust 1.98.0、Meson 1.12.1、
Ninja 1.13.2、CMake 4.4.3；Python 3.14。Rust target 为 `wasm32-wasip1`。
此前栅格 `native/build.py` 的开发缓存须已有 SHA256 验证的 zlib 静态产物。
以下参数路径可按本机调整，构建 cache 必须不含空白（libffi configure 限制），
仓库和 SDK 路径可含空白。

```sh
python3 internal/stellarimages/native/librsvg/build.py \
  --cache /tmp/stellar-librsvg-rebuild \
  --wasi-sdk .tmp/image-native-build/wasi-sdk-29.0-arm64-macos \
  --archives .tmp/image-native-build/official-svg \
  --raster-cache .tmp/image-native-build \
  --meson .tmp/image-native-build/svg-venv/bin/meson
```

加 `--check` 会重新构建并逐字比较已提交 `librsvg.wasm`，不覆盖它。
使用新 cache 可以从所有锁定上游原始归档验证完整图形链。
开发期缺失归档才下载；每个归档在解包前验证 SHA256，Rust 使用 `--locked`。
构建不会覆盖已有 GLib 测试夹具。编译期默认目录与文件前缀固定为虚拟路径；
外部 C archives 不在 Cargo 的依赖追踪里，因此每次强制重链 renderer 小包，
保留其余 crate cache。两套独立源码 cache 的产物已经逐字核对一致。
无需在生产启动时构建或下载。

```sh
cargo fetch --locked --manifest-path internal/stellarimages/native/librsvg/Cargo.toml
python3 internal/stellarimages/native/librsvg/collect-licenses.py
go test -race ./internal/stellarimages/...
go run ./internal/stellarimages/oracle
```

oracle 全部夹具逐字段精确比较平均色、透明度和尺寸，没有放宽容差。
