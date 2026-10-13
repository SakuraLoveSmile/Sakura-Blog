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

## GLib/GObject/GIO 实际运行

原版 GLib2.86.1、GObject 与 GIO 的同步内存流／转换器现已构建为 WASI，真实 Wazero 测试通过，不访问宿主文件系统。`wasi/build-glib.py` 从 SHA256 锁定的 GLib／PCRE2／libffi 原源码重建；`wasi/glib-wasi.patch` 保存平台差异。全新 `/tmp/stellar-glib-rebuild` 构建与 `--check` 字节比较通过。

实际断言覆盖正则、GObject 对象创建／信号 libffi 双精度回调、gzip 转换内存流／UTF-8 读取、虚拟账号、UTC 时区、进程／网络拒绝、无法创建线程和取消状态。WASI 的单实例非共享内存使用官方 GLib 互斥版原子操作及 SDK 单线程 pthread 语义；不能创建 worker，`g_thread_try_new` 返回真实错误。

GLib 原生 C 类型初始化允许单参数函数忽略第二个数据参数，WASM 则严格检查签名。类型初始化新增独立桥，仅接受一个／两个 i32 参数且无返回值的固定模块函数，通用 libffi CIF 检查仍严格。链表 `free_full` 使用原始单参数析构器而不强转成双参数遍历函数。非法返回签名有拒绝测试。

`wasi/platform.c` 对 fork／exec／pipe／wait／socket／DNS 返回 ENOSYS；不存在宿主账户或进程，固定虚拟账号为 stellar／1000，时区固定 UTC。pthread_exit 终止错误调用的实例。Expat 使用 WASI getentropy 获取 XML hash 熵源；原生 codec 致命错误终止隔离实例，不能把 longjmp 变成成功继续。

GIO 目前仅构建已验证的同步内存流、转换器、属性和任务基础。未编入 Unix 桌面、DBus、网络及可写磁盘 backend；**还没有完成 librsvg 所需 GFile／资源接入或 GMainContext 的受限唤醒实现**。不能宣称完整 GIO 等效。

重建命令：

```sh
python3 internal/stellarimages/native/wasi/build-glib.py \
  --cache /tmp/stellar-glib-rebuild \
  --wasi-sdk .tmp/image-native-build/wasi-sdk-29.0-arm64-macos \
  --archives .tmp/image-native-build/official-svg \
  --raster-cache .tmp/image-native-build \
  --meson .tmp/image-native-build/svg-venv/bin/meson [--check]
go test -race ./internal/stellarimages/wasmffi
```

## 图形链仍在实现

开发现场 `/tmp/stellar-svg-wasi-58ecec7` 已构建 pixman0.46.4、FriBidi1.0.16、FreeType2.14.1、libpng1.6.50、Expat2.7.3、Fontconfig2.17.1 静态 archives；这仅是编译证据，还未作为最终渲染链运行。Cairo1.18.4 与 Harfbuzz12.1.0 在编译中，Pango1.57.0／librsvg2.61.2 尚未完成。后续真实符号、文件／字体和回调问题必须继续实现和验证。

SVG 生产仍为过渡 resvg，complex.svg Sharp 黄金精确对照保持失败；本阶段没有替换运行态，也未更改数值容差、黄金夹具或冻结 Go 接口。原版渲染路线已获父任务确认；不新增 Sharp/NAPI/Node 生产依赖。
