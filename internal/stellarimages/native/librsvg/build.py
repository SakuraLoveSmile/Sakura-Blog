"""Build the embedded librsvg renderer from checksum-locked sources.

Development only: Python, Rust, Meson, Ninja, CMake and WASI SDK never run
in production. Use a new, whitespace-free cache to verify the complete chain.
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
NATIVE = HERE.parent
WASI = NATIVE / 'wasi'
parser = argparse.ArgumentParser()
parser.add_argument('--cache', type=Path, required=True)
parser.add_argument('--wasi-sdk', type=Path, required=True)
parser.add_argument('--archives', type=Path, required=True)
parser.add_argument('--raster-cache', type=Path, required=True)
parser.add_argument('--meson', type=Path, required=True)
parser.add_argument('--check', action='store_true')
args = parser.parse_args()
cache = args.cache.absolute()
if any(c.isspace() for c in str(cache)):
    raise SystemExit('libffi development cache must not contain whitespace')
cache.mkdir(parents=True, exist_ok=True)
sdk = args.wasi_sdk.resolve()
archives = args.archives.resolve()
archives.mkdir(parents=True, exist_ok=True)
meson = args.meson.resolve()
env = dict(os.environ)
env['PATH'] = str(meson.parent) + os.pathsep + env['PATH']
prefix = cache / 'prefix'
env.update(PKG_CONFIG_LIBDIR=str(prefix / 'lib/pkgconfig'),
           PKG_CONFIG_ALLOW_CROSS='1', PKG_CONFIG_ALL_STATIC='1')


def run(*argv, cwd=None):
    subprocess.run([str(a) for a in argv], cwd=cwd, env=env, check=True)


def extract(row, destination):
    archive = archives / row['archive']
    if not archive.exists() and archive.suffix == '.crate':
        cargo = Path(os.environ.get('CARGO_HOME', str(Path.home() / '.cargo')))
        cached = list((cargo / 'registry/cache').glob('*/' + archive.name))
        if cached:
            shutil.copyfile(cached[0], archive)
    if not archive.exists():
        with urllib.request.urlopen(row['url'], timeout=90) as response:
            archive.write_bytes(response.read())
    if hashlib.sha256(archive.read_bytes()).hexdigest() != row['sha256']:
        raise SystemExit('source digest mismatch: ' + archive.name)
    with tarfile.open(archive) as tar:
        root = tar.getmembers()[0].name.split('/')[0]
        if not (destination / root).exists():
            tar.extractall(destination, filter='data')
    return destination / root


def patch(source, file):
    marker = source / ('.stellar-' + file.name)
    digest = hashlib.sha256(file.read_bytes()).hexdigest()
    if marker.exists():
        if marker.read_text() != digest:
            raise SystemExit('patch changed; use a new cache: ' + file.name)
        return
    run('patch', '-p1', '--input', file, cwd=source)
    marker.write_text(digest)


# The stream fixture also builds the real GLib/GObject/GIO archives and FFI.
# --output keeps this renderer build from changing a separately tracked fixture.
run('python3', WASI / 'build-glib.py', '--cache', cache,
    '--wasi-sdk', sdk, '--archives', archives,
    '--raster-cache', args.raster_cache.resolve(), '--meson', meson,
    '--output', cache / 'glib-fixture.wasm')
source = cache / 'src'
sources = {}
for row in json.loads((HERE / 'build-sources.json').read_text()):
    path = extract(row, source)
    sources[row['archive'].split('-')[0]] = path

# Install only built GLib libraries, public headers and their pkg-config files.
# Desktop programs and girepository are intentionally not renderer dependencies.
glibbuild = cache / 'glib-build'
run('ninja', '-C', glibbuild, 'gmodule/libgmodule-2.0.a', 'gthread/libgthread-2.0.a')
installed = json.loads((glibbuild / 'meson-info/intro-installed.json').read_text())
pc_names = {'glib-2.0.pc', 'gobject-2.0.pc', 'gthread-2.0.pc',
            'gmodule-2.0.pc', 'gmodule-export-2.0.pc',
            'gmodule-no-export-2.0.pc', 'gio-2.0.pc'}
for original, target in installed.items():
    original, target = Path(original), Path(target)
    if (original.suffix == '.h' and 'girepository' not in str(original)
            or original.name in pc_names
            or original.name in {'glib-mkenums', 'glib-genmarshal',
                                 'libglib-2.0.a', 'libgobject-2.0.a', 'libgio-2.0.a',
                                 'libgmodule-2.0.a', 'libgthread-2.0.a', 'libintl.a'}):
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(original, target)
        if original.name in {'glib-mkenums', 'glib-genmarshal'}:
            target.chmod(0o755)

# C++ needs the same single-thread WASI/platform declarations as C.
wrapper = cache / 'clangpp-meson'
wrapper.write_text('#!/usr/bin/env python3\nimport os,sys\n' +
    'args=[a for a in sys.argv[1:] if a not in ["-Wl,--start-group","-Wl,--end-group","-pthread"]]\n' +
    'args += ' + repr(['-D_WASI_EMULATED_MMAN', '-D_WASI_EMULATED_SIGNAL',
                       '-I' + str(WASI / 'include')]) + '\n' +
    'if "-c" not in args and "-E" not in args: args += ' +
    repr(['-L' + str(prefix / 'lib'), '-lstellar-wasi']) + '\n' +
    'os.execv(' + repr(str(sdk / 'bin/clang++')) + ',["clang++"]+args)\n')
wrapper.chmod(0o755)
cross = cache / 'renderer-meson.ini'
cross.write_text('\n'.join(
    "cpp = '" + str(wrapper) + "'" if line.startswith('cpp = ') else line
    for line in (cache / 'wasi-meson.ini').read_text().splitlines()) + '\n')


def cmake(name, options, target=None):
    build = cache / (name + '-build')
    run('cmake', '-S', sources[name], '-B', build,
        '-DCMAKE_TOOLCHAIN_FILE=' + str(sdk / 'share/cmake/wasi-sdk.cmake'),
        '-DCMAKE_INSTALL_PREFIX=' + str(prefix),
        '-DCMAKE_C_FLAGS=-I"' + str(WASI / 'include') + '"', *options)
    command = ['cmake', '--build', build, '--parallel', '8']
    if target:
        command.extend(['--target', target])
    run(*command)
    run('cmake', '--install', build)


def meson_build(name, options, targets=None, buildtype='debugoptimized'):
    build = cache / (name + '-build')
    run(meson, 'setup', *(['--reconfigure'] if (build / 'build.ninja').exists() else []),
        build, sources[name], '--prefix', prefix, '--cross-file', cross,
        '--default-library', 'static', '-Dprefer_static=true', '--buildtype', buildtype, *options)
    run('ninja', '-C', build, *(targets or []))
    run(meson, 'install', '-C', build, '--no-rebuild')


meson_build('pixman', ['-Dtests=disabled', '-Ddemos=disabled',
                     '-Dgtk=disabled', '-Dlibpng=disabled'])
meson_build('fribidi', ['-Ddocs=false', '-Dbin=false', '-Dtests=false'], buildtype='debug')
cmake('freetype', ['-DFT_DISABLE_ZLIB=ON', '-DFT_DISABLE_BZIP2=ON',
                  '-DFT_DISABLE_PNG=ON', '-DFT_DISABLE_HARFBUZZ=ON', '-DFT_DISABLE_BROTLI=ON'])
cmake('libpng', ['-DPNG_SHARED=OFF', '-DPNG_TESTS=OFF', '-DPNG_TOOLS=OFF',
                '-DPNG_EXECUTABLES=OFF', '-DZLIB_LIBRARY=' + str(prefix / 'lib/libz.a'),
                '-DZLIB_INCLUDE_DIR=' + str(prefix / 'include')])
# libpng's unknown-platform CMake branch installs a differently named archive,
# while its upstream pkg-config still requests png16. Expose that same archive.
shutil.copyfile(prefix / 'lib/liblibpng16_static.a', prefix / 'lib/libpng16.a')
# Expat's XML hash seed uses the real WASI entropy function from platform.c.
cmake('expat', ['-DEXPAT_SHARED_LIBS=OFF', '-DEXPAT_BUILD_TOOLS=OFF',
               '-DEXPAT_BUILD_EXAMPLES=OFF', '-DEXPAT_BUILD_TESTS=OFF',
               '-DCMAKE_C_FLAGS=-DHAVE_ARC4RANDOM_BUF -I"' + str(WASI / 'include') + '"'])
patch(sources['fontconfig'], HERE / 'fontconfig-wasi.patch')
meson_build('fontconfig', ['-Ddoc=disabled', '-Dnls=disabled', '-Dtests=disabled',
                          '-Dtools=disabled', '-Dcache-build=disabled',
                          '-Diconv=disabled', '-Dfontations=disabled'])
meson_build('cairo', ['-Dfontconfig=enabled', '-Dfreetype=enabled', '-Dpng=enabled',
                     '-Dquartz=disabled', '-Dxcb=disabled', '-Dxlib=disabled',
                     '-Dxlib-xcb=disabled', '-Dzlib=disabled', '-Dtests=disabled',
                     '-Dgtk2-utils=disabled', '-Dglib=enabled', '-Dspectre=disabled',
                     '-Dsymbol-lookup=disabled', '-Dgtk_doc=false'], buildtype='debug')
meson_build('harfbuzz', ['-Dglib=disabled', '-Dgobject=disabled', '-Dcairo=disabled',
                        '-Dicu=disabled', '-Dgraphite=disabled', '-Dgraphite2=disabled',
                        '-Dfreetype=enabled', '-Dfontations=disabled', '-Dgdi=disabled',
                        '-Ddirectwrite=disabled', '-Dcoretext=disabled', '-Dharfrust=disabled',
                        '-Dkbts=disabled', '-Dwasm=disabled', '-Dtests=disabled',
                        '-Dintrospection=disabled', '-Ddocs=disabled', '-Dutilities=disabled',
                        '-Dbenchmark=disabled', '-Dwith_libstdcxx=false'])
patch(sources['pango'], HERE / 'pango-wasi.patch')
meson_build('pango', ['-Ddocumentation=false', '-Dgtk_doc=false', '-Dman-pages=false',
                     '-Dintrospection=disabled', '-Dbuild-testsuite=false',
                     '-Dbuild-examples=false', '-Dfontconfig=enabled', '-Dsysprof=disabled',
                     '-Dlibthai=disabled', '-Dcairo=enabled', '-Dxft=disabled', '-Dfreetype=enabled'])
cmake('libxml2', ['-DBUILD_SHARED_LIBS=OFF', '-DLIBXML2_WITH_THREADS=OFF',
                 '-DLIBXML2_WITH_HTTP=OFF', '-DLIBXML2_WITH_PYTHON=OFF',
                 '-DLIBXML2_WITH_ICONV=OFF', '-DLIBXML2_WITH_ZLIB=OFF',
                 '-DLIBXML2_WITH_PROGRAMS=OFF', '-DLIBXML2_WITH_TESTS=OFF',
                 '-DLIBXML2_WITH_DOCS=OFF'])

# Buffer-only SVG external URI identity; no host/network/file-reading GIO backend.
cc = cache / 'clang-meson'
includes = [source / 'glib-2.86.1', source / 'glib-2.86.1/gio', glibbuild,
            glibbuild / 'glib', prefix / 'include/glib-2.0']
objects = []
for file in [WASI / 'renderer-file.c', source / 'glib-2.86.1/gio/gdummyfile.c']:
    obj = cache / (file.stem + '.o')
    run(cc, '-O2', '-D_WASI_EMULATED_SIGNAL',
        *['-I' + str(i) for i in includes], '-c', file, '-o', obj)
    objects.append(obj)
run(sdk / 'bin/llvm-ar', 'rcs', prefix / 'lib/libstellar-wasi.a', *objects)

rust = cache / 'renderer-rust'
rust.mkdir(exist_ok=True)
for name in ['Cargo.toml', 'Cargo.lock']:
    shutil.copyfile(HERE / name, rust / name)
shutil.copytree(HERE / 'src', rust / 'src', dirs_exist_ok=True)
if not (rust / '_source').exists():
    (rust / '_source').symlink_to(sources['librsvg'], target_is_directory=True)
ports = rust / '_rust'
ports.mkdir(exist_ok=True)
for row in json.loads((HERE / 'rust-ports.json').read_text()):
    extracted = extract(row, ports)
    name = ports / row['name']
    if not name.exists():
        extracted.rename(name)
    patch(name, HERE / row['patch'])
env['CC_wasm32_wasip1'] = str(cc)
env['CXX_wasm32_wasip1'] = str(wrapper)
env['CARGO_ENCODED_RUSTFLAGS'] = '\x1f'.join([
    '-C', 'link-arg=-L' + str(prefix / 'lib'), '-C', 'link-arg=-lstellar-wasi',
    '-C', 'link-arg=-L' + str(sdk / 'share/wasi-sysroot/lib/wasm32-wasi'),
    *sum([['-C', 'link-arg=-lwasi-emulated-' + name]
          for name in ['signal', 'mman', 'process-clocks', 'getpid']], []),
    '-C', 'link-arg=--export-table', '-C', 'link-arg=--max-memory=536870912',
    '--remap-path-prefix=' + str(cache) + '=/stellar-build'])
run('cargo', 'build', '--release', '--target', 'wasm32-wasip1', '--locked', cwd=rust)
output = cache / 'librsvg.wasm'
shutil.copyfile(rust / 'target/wasm32-wasip1/release/stellar-librsvg.wasm', output)
run(sdk / 'bin/llvm-strip', '--strip-debug', output)
target = NATIVE / 'librsvg.wasm'
if args.check:
    if output.read_bytes() != target.read_bytes():
        raise SystemExit('embedded librsvg renderer drift')
else:
    shutil.copyfile(output, target)
    target.chmod(0o644)
print('Embedded librsvg verified' if args.check else 'Embedded librsvg rebuilt')
