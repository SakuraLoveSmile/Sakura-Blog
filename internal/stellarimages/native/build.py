"""Rebuild embedded decoders with WASI SDK 29, Rust 1.98 and CMake.

Only a development tool. The Go application never invokes this script, Rust,
CMake, Node or a host image library. Downloaded archives are hash checked.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import urllib.request

HERE = Path(__file__).resolve().parent
parser = argparse.ArgumentParser()
parser.add_argument("--cache", type=Path, required=True)
parser.add_argument("--wasi-sdk", type=Path, required=True)
parser.add_argument("--check", action="store_true")
args = parser.parse_args()
cache = args.cache.resolve()
cache.mkdir(parents=True, exist_ok=True)
sdk = args.wasi_sdk.resolve()
cc = sdk / "bin/clang"
toolchain = sdk / "share/cmake/wasi-sdk.cmake"
sources = json.loads((HERE / "build-sources.json").read_text())


def command(*argv, cwd=None):
    subprocess.run([str(x) for x in argv], cwd=cwd, check=True)


for row in sources:
    if row["archive"] == "wasi-sdk.tar.gz":
        continue  # The caller selects the SDK for their development host.
    archive = cache / row["archive"]
    if not archive.exists():
        with urllib.request.urlopen(row["url"], timeout=90) as response:
            archive.write_bytes(response.read())
    digest = hashlib.sha256(archive.read_bytes()).hexdigest()
    if digest != row["sha256"]:
        raise SystemExit(f"source digest mismatch: {archive.name}")
    with tarfile.open(archive) as tar:
        root = tar.getmembers()[0].name.split("/")[0]
        if not (cache / root).exists():
            tar.extractall(cache, filter="data")

prefix = cache / "prefix"


def cmake(source, build, target, options):
    command("cmake", "-S", cache / source, "-B", cache / build,
            f"-DCMAKE_TOOLCHAIN_FILE={toolchain}",
            f"-DCMAKE_INSTALL_PREFIX={prefix}", *options)
    command("cmake", "--build", cache / build, "--target", target, "--parallel", "8")


cmake("zlib-1.3.1", "zlib-build", "zlibstatic", ["-DBUILD_SHARED_LIBS=OFF"])
# Do not build/install host command-line programs or cross-compiled test tools.
(prefix / "include").mkdir(parents=True, exist_ok=True)
(prefix / "lib").mkdir(exist_ok=True)
for name in ["zlib.h"]:
    shutil.copy2(cache / "zlib-1.3.1" / name, prefix / "include" / name)
shutil.copy2(cache / "zlib-build/zconf.h", prefix / "include/zconf.h")
shutil.copy2(cache / "zlib-build/libzlibstatic.a", prefix / "lib/libzlibstatic.a")
cmake("libjpeg-turbo-3.1.2", "jpeg-build", "jpeg-static", [
    "-DENABLE_SHARED=OFF", "-DENABLE_STATIC=ON", "-DWITH_SIMD=OFF", "-DWITH_TURBOJPEG=OFF"])
for name in ["jpeglib.h", "jmorecfg.h", "jerror.h"]:
    shutil.copy2(cache / "libjpeg-turbo-3.1.2/src" / name, prefix / "include" / name)
shutil.copy2(cache / "jpeg-build/jconfig.h", prefix / "include/jconfig.h")
shutil.copy2(cache / "jpeg-build/libjpeg.a", prefix / "lib/libjpeg.a")
cmake("mozjpeg-0826579", "mozjpeg-build", "jpeg-static", [
    "-DBUILD_SHARED_LIBS=OFF", "-DWITH_SIMD=OFF", "-DWITH_TURBOJPEG=OFF", "-DPNG_SUPPORTED=OFF",
    "-DWITH_ARITH_DEC=ON"])
cmake("libwebp-1.6.0", "webp-build", "webp", [
    "-DBUILD_SHARED_LIBS=OFF", "-DWEBP_USE_THREAD=OFF", "-DWEBP_ENABLE_SIMD=OFF",
    "-DWEBP_BUILD_ANIM_UTILS=OFF", "-DWEBP_BUILD_CWEBP=OFF", "-DWEBP_BUILD_DWEBP=OFF",
    "-DWEBP_BUILD_GIF2WEBP=OFF", "-DWEBP_BUILD_IMG2WEBP=OFF", "-DWEBP_BUILD_VWEBP=OFF",
    "-DWEBP_BUILD_WEBPINFO=OFF", "-DWEBP_BUILD_EXTRAS=OFF", "-DWEBP_BUILD_WEBPMUX=OFF"])
command("cmake", "--build", cache / "webp-build", "--target", "webpdemux", "--parallel", "8")
cmake("tiff-4.7.1", "tiff-build", "tiff", [
    "-DBUILD_SHARED_LIBS=OFF", "-Dtiff-tools=OFF", "-Dtiff-tests=OFF", "-Dtiff-contrib=OFF",
    "-Dtiff-docs=OFF", "-Dtiff-install=OFF", f'-DCMAKE_C_FLAGS=-I"{HERE}"',
    f"-DZLIB_LIBRARY={prefix / 'lib/libzlibstatic.a'}", f"-DZLIB_INCLUDE_DIR={prefix / 'include'}",
    f"-DJPEG_LIBRARY={prefix / 'lib/libjpeg.a'}", f"-DJPEG_INCLUDE_DIR={prefix / 'include'}",
    "-Dwebp=ON", f"-DWebP_LIBRARY={cache / 'webp-build/libwebp.a'}",
    f"-DWebP_INCLUDE_DIR={cache / 'libwebp-1.6.0/src'}", "-Dlzma=OFF", "-Dzstd=OFF",
    "-Djbig=OFF", "-Dlerc=OFF"])

output = cache / "artifacts"
output.mkdir(exist_ok=True)
link = ["-lm", "-Wl,--initial-memory=16777216", "-Wl,--max-memory=1073741824"]


def compile_c(name, includes, libraries, additional=()):
    command(cc, "-O2", *[f"-I{x}" for x in includes], HERE / f"{name}.c",
            *additional, *libraries, *link, "-o", output / f"{name}.wasm")


lcms = cache / "Little-CMS-lcms2.17"
compile_c("color", [lcms / "include"], [], sorted((lcms / "src").glob("*.c")))
compile_c("jpeg", [cache / "mozjpeg-build", cache / "mozjpeg-0826579"], [cache / "mozjpeg-build/libjpeg.a"])
webp = [cache / "webp-build/libwebp.a", cache / "webp-build/libsharpyuv.a"]
compile_c("webp", [cache / "libwebp-1.6.0/src"], [cache / "webp-build/libwebpdemux.a", *webp])
compile_c("tiff", [cache / "tiff-4.7.1/libtiff", cache / "tiff-build/libtiff"], [
    cache / "tiff-build/libtiff/libtiff.a", prefix / "lib/libjpeg.a", prefix / "lib/libzlibstatic.a", *webp])
command("cargo", "build", "--release", "--locked", "--target", "wasm32-wasip1", cwd=HERE / "svg")
shutil.copy2(HERE / "svg/target/wasm32-wasip1/release/stellar-svg.wasm", output / "svg.wasm")
for file in sorted(output.glob("*.wasm")):
    target = HERE / file.name
    if args.check:
        if file.read_bytes() != target.read_bytes():
            raise SystemExit(f"embedded decoder drift: {file.name}")
    else:
        shutil.copyfile(file, target)
        target.chmod(0o644)
print("Embedded decoder verification passed" if args.check else "Embedded decoders rebuilt")
