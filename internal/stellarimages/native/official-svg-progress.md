# 官方 SVG WASI 移植现场

当前正式解码路径已接通 librsvg 2.61.2 WASI，生产保持 Go 单二进制。
这份记录只覆盖 `internal/stellarimages`；自动 SSR 生命周期、CLI 及整个前台验收由主实现树独立验证。

## 当前交付

- `decodeSVG` 嵌入 `librsvg.wasm`，每图独立 WASI 实例；内存 512 MiB、实例时限 15 秒、输入 32 MiB、原尺寸 40 MP、取样最长边 64。
- 完整原版 GLib/GObject/GIO 内存流、Cairo、Pango、Fontconfig、Harfbuzz、FreeType、librsvg 链。没有 Node、Sharp、宿主图片库、宿主字体或宿主网络运行依赖。
- 字体已接入嵌入的 Noto Sans／Noto Sans SC；默认 sans-serif 会走中文 fallback。Fontconfig 仅读取虚拟只读字体与配置，不写磁盘缓存。
- Pango 的字体初始化、匹配与排序同步运行同一官方函数，WASI 不创建线程。修复 `pango_attribute_copy` 单参函数转双参 GCopyFunc、单参析构转双参 g_slist_foreach 导致的真实 signature trap；未放宽 WASM 类型或 oracle 容差。
- 自然尺寸按 libvips：CSS 像素 → viewBox/单边比例 → 可见几何，支持 SVGZ 的官方 GIO 解压。尺寸／损坏输入有拒绝测试。
- GFile 使用官方 GDummyFile URI 身份；外部资源读取明确拒绝。实际测试证明 SVG 无法读取宿主文件或请求宿主网络。
- PageSnapshot 可选 BaseURL，非空只接受 absolute HTTP(S)，按真实无尾斜杠页面地址解析相对图；空值仍按 Path 的官方规则。Path 继续作为 --page 的输出 key。
- 取消批次不会改已有 metadata、记图片失败或产生 24h backoff。HEAD 红基线曾计 Processed=1/Failed=1；修复后的定向测试断言字节不变、立即下一批可处理、已取消批次不创建目录。

## 来源和重建

完整来源指纹、Rust 锁、平台补丁、原生及 crate 许可证在 [librsvg/README.md](librsvg/README.md)。
`librsvg/build.py` 从锁定原始归档重建全部图形链，使用 `--locked` 的 Rust；`--check` 逐字比较产物。
开发工具链为 WASI SDK29 / Clang21.1.4、Rust1.98.0、Meson1.12.1、Ninja1.13.2、CMake4.4.3。

第一个修复现场 cache 曾在缺少静态 png/zlib 依赖时缓存 Cairo 的负链接探测结果。
其后 reconfigure 没清除那些结果，独立 cache 比较发现实际 WASM 漂移；没有将它写成可复现通过。
当前嵌入产物由冻结完整 recipe 在两个独立原始源码 cache 重建后逐字一致。
另修复 Fontconfig/GLib/libxml2 的编译期目录字符串和 __FILE__ 造成的路径漂移：
从配置头和编译宏层固定虚拟目录，没有后处理 WASM。Cargo 不会追踪外部 C archives，
构建脚本因此强制重链小型 renderer，保留依赖 crate cache，避免旧链接假通过。
最终 SHA256：`d254aaa66ab23393a76492428b2baa967d6f298ee73e597c8b25585ccd065000`。

## 本次证据

- 21/21 Sharp 黄金逐字段精确比较，SVG 已纳入普通 `TestSharpOracle`，不再只跑栅格。
- 正式字体、invalid/viewBox、宿主文件／网络拒绝和取消回归已通过。
- `go test -race ./internal/stellarimages/...` 完整纯净 Cairo 产物通过；目录规范化后全部包普通检查和精确 oracle 再次通过。
- 清空 PATH 等宿主环境的独立 test binary 跑 Sharp/SVG 通过。
- CGO0 Linux/amd64 images test 构建产生包含完整 librsvg WASM 的静态 ELF。此旧分支 cmd/blog 还没有 CLI/自动生命周期引用，链接器会删去 decoder；不能将其 Blog 构建当主应用图片验收。
- BaseURL 提交 `4cf310e`、正式 decoder/取消提交 `7f8739a` 都通过正常完整 pre-commit make test；规范化最终产物随后按同一完整门禁提交。

这些结果不是整个 T6 自动生命周期或 T7 浏览器验收完成的证明。
