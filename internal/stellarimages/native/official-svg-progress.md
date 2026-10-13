# 官方 SVG WASI 移植现场

仍为实现中，未产生可运行的 librsvg SVG 解码器。不得将栅格门禁或标量 FFI 测试写成 SVG 等效完成。

## 已交付基础

`58ecec7` 为 Go 图片生命周期及嵌入栅格解码器；正常 pre-commit 完整 make test 通过。`testdata/sharp.json` 与全量 oracle 保留复杂 SVG 失败，当前 21 个夹具 20 个精确一致。Linux CGO0 单静态 ELF 编译和无 PATH/DYLD_LIBRARY_PATH/NODE_PATH 的 host 测试执行通过。

## 平台 ABI 桥

官方 libffi 3.5.2 的 WASM 目标直接包含 `emscripten/emscripten.h`，`ffi_prep_cif_machdep` 只接受 `FFI_WASM32_EMSCRIPTEN`；原始 `FFI_WASM32` ABI 未实现。无空格目录下官方 configure 成功，但 WASI SDK29 编译在该头文件失败。

GLib `gobject/gclosure.c` 只使用 `ffi_prep_cif`、`ffi_call`，不使用 libffi 动态 closure。新增 `ffi-wasi.c` 保留官方通用 CIF/type 源，标量调用导入 Go `wasmffi`；后者给固定函数表增加导出别名，按 WASM 真实参数／返回签名调用，不变更函数指令／签名／数据。已通过真实 WASI 的 signed8/16/32、uint64/int64、float/double、pointer、void 回调及 race；非 export 段逐字保留。未接入图片运行态。

开发现场路径（均为隔离开发缓存，不是生产依赖）：

- 项目 `.tmp/image-native-build/official-svg/`：已获取 GLib2.86.1、librsvg2.61.2、Pango1.57.0、Cairo1.18.4、pixman0.46.4、libffi3.5.2、Expat2.7.3、PCRE2 10.44 源；部分后续字体库也已下载。
- 项目 `.tmp/image-native-build/svg-venv/`：Meson1.12.1、Ninja1.13.2。
- `/tmp/stellar-svg-wasi-58ecec7/`：无空格开发别名，`src` 链接上述源，`sdk` 链接 WASI SDK29。
- `ffi-wasi/libffi.a`：官方 prep_cif.c/types.c＋新桥；`libffi-build/include/ffi.h`／ffitarget.h 为官方 configure 生成头。
- `prefix/`：已安装 PCRE2 10.44，复制 zlib1.3.1 和标量 libffi。
- `wasi-meson.ini`／`clang-meson`：交叉配置；wrapper 去掉 Meson 误加的 GNU `--start-group/--end-group`，WASM ld 自动处理 archive 依赖；配置真实成功，未强行伪造函数探测结果。

FFI 夹具重建：

```sh
SDK=/tmp/stellar-svg-wasi-58ecec7/sdk
FFI=/tmp/stellar-svg-wasi-58ecec7/libffi-build
SRC=/tmp/stellar-svg-wasi-58ecec7/src/libffi-3.5.2
OUT=/tmp/stellar-svg-wasi-58ecec7/ffi-wasi
# clang -O2 -DHAVE_CONFIG_H -I$FFI -I$FFI/include -I$SRC/include
# 分别编译 $SRC/src/prep_cif.c、types.c 与 native/ffi-wasi.c，再 llvm-ar rcs libffi.a。
$SDK/bin/clang -O2 -I$FFI/include internal/stellarimages/native/ffi-fixture.c \
  $OUT/libffi.a -Wl,--export-table -o internal/stellarimages/wasmffi/testdata/scalar.wasm
go test -race ./internal/stellarimages/wasmffi
```

## GLib 当前实际阻塞

原生 WASI 不提供 GIO 所需 `resolv.h/res_query/socket`。隔离原型提供 **拒绝网络、返回 ENOSYS** 的声明／实现，没有把网络调用伪装成成功；SVG 解码器的网络本就应封闭，顶层抓取由 Go safefetch 完成。该适配只在开发缓存，尚未提交为生产端口。

`glib-build-network-denied` 的 Meson 配置已真实通过。`ninja -k 0` 已完成可编译对象，但不能生成三个完整库；完整失败日志：`/tmp/stellar-svg-wasi-58ecec7/glib-platform-errors.log`。

仍缺的平台边界包括：`sys/wait.h`、`pwd.h`、`netdb.h`；pipe/fork/exec/wait、getuid/getppid、tzset、pthread_exit、CPU affinity 宏。涉及 gbacktrace/gspawn、gutils/gdate/gthread、GIO 网络及 Unix 私有头。不能只加空成功 stub 让配置变绿。下一步应对每个边界实现有明确能力约束的 WASI 端口：无宿主进程／账户／网络访问；只有渲染所需的内存流、虚拟字体文件系统、单实例线程语义可接入；随后验证官方 GLib/GObject/GIO 实际程序，继续 Cairo/Pango/librsvg 和严格 SVG oracle。

原版渲染路线已获父任务确认；不新增 Sharp/NAPI/Node 生产依赖、不改数值容差、不修改冻结 Go 接口。
