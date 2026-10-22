"""Build the verified GLib/GObject/GIO stream WASI fixture.

Development only. The application never runs this script or host libraries.
The cache must have no spaces because libffi's configure rejects them.
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
parser = argparse.ArgumentParser()
parser.add_argument('--cache', type=Path, required=True)
parser.add_argument('--wasi-sdk', type=Path, required=True)
parser.add_argument('--archives', type=Path, required=True)
parser.add_argument('--raster-cache', type=Path, required=True)
parser.add_argument('--meson', type=Path, required=True)
parser.add_argument('--check', action='store_true')
parser.add_argument('--output', type=Path)
args = parser.parse_args()
cache = args.cache.absolute()
if any(c.isspace() for c in str(cache)):
    raise SystemExit('libffi development cache must not contain whitespace')
cache.mkdir(parents=True, exist_ok=True)
sdk_target = args.wasi_sdk.resolve()
sdk = cache/'sdk'
if not sdk.exists():
    sdk.symlink_to(sdk_target, target_is_directory=True)
elif sdk.resolve() != sdk_target:
    raise SystemExit('cache belongs to another WASI SDK')
archives = args.archives.resolve()
archives.mkdir(parents=True, exist_ok=True)
raster = args.raster_cache.resolve()
meson = args.meson.resolve()
env = dict(os.environ)
env['PATH'] = str(meson.parent) + os.pathsep + env['PATH']
prefix = cache/'prefix'
env['PKG_CONFIG_LIBDIR'] = str(prefix/'lib/pkgconfig')


def run(*argv, cwd=None, extra_env=None):
    subprocess.run([str(a) for a in argv], cwd=cwd, env=extra_env or env, check=True)


source = cache/'src'
source.mkdir(exist_ok=True)
rows = json.loads((HERE/'build-sources.json').read_text())
for row in rows:
    if row['archive'].startswith(('fontconfig-', 'harfbuzz-')):
        continue  # Later rendering stages are not part of this stream fixture.
    archive = archives/row['archive']
    if not archive.exists():
        with urllib.request.urlopen(row['url'], timeout=50) as response:
            archive.write_bytes(response.read())
    if hashlib.sha256(archive.read_bytes()).hexdigest() != row['sha256']:
        raise SystemExit('source digest mismatch: '+archive.name)
    with tarfile.open(archive) as tar:
        root = tar.getmembers()[0].name.split('/')[0]
        if not (source/root).exists():
            tar.extractall(source, filter='data')

(prefix/'include').mkdir(parents=True, exist_ok=True)
(prefix/'lib/pkgconfig').mkdir(parents=True, exist_ok=True)
for name in ['zlib.h','zconf.h']:
    shutil.copyfile(raster/'prefix/include'/name, prefix/'include'/name)
shutil.copyfile(raster/'prefix/lib/libzlibstatic.a',prefix/'lib/libz.a')
(prefix/'lib/pkgconfig/zlib.pc').write_text(f'''prefix={prefix}
libdir=${{prefix}}/lib
includedir=${{prefix}}/include
Name: zlib
Description: zlib compression
Version: 1.3.1
Libs: -L${{libdir}} -lz
Cflags: -I${{includedir}}
''')

cc = sdk/'bin/clang'
ar = sdk/'bin/llvm-ar'
port = cache/'platform.o'
run(cc,'-O2','-D_WASI_EMULATED_SIGNAL','-I',HERE/'include','-c',HERE/'platform.c','-o',port)
run(ar,'rcs',prefix/'lib/libstellar-wasi.a',port)
wrapper = cache/'clang-meson'
wrapper.write_text('#!/usr/bin/env python3\nimport os,sys\n'+
    'args=[a for a in sys.argv[1:] if a not in ["-Wl,--start-group","-Wl,--end-group","-pthread"]]\n'+
    'args += '+repr(['-I'+str(HERE/'include')])+'\n'+
    'if "-c" not in args and "-E" not in args: args += '+repr(['-L'+str(prefix/'lib'),'-lstellar-wasi'])+'\n'+
    'os.execv('+repr(str(cc))+',["clang"]+args)\n')
wrapper.chmod(0o755)
cross = cache/'wasi-meson.ini'
cross.write_text(f'''[binaries]
c = '{wrapper}'
cpp = '{sdk}/bin/clang++'
ar = '{ar}'
strip = '{sdk}/bin/llvm-strip'
pkg-config = '{shutil.which('pkg-config')}'
[host_machine]
system = 'wasi'
cpu_family = 'wasm32'
cpu = 'wasm32'
endian = 'little'
[properties]
needs_exe_wrapper = true
[built-in options]
c_args = ['-D_WASI_EMULATED_SIGNAL', '-D_WASI_EMULATED_MMAN', '-D_WASI_EMULATED_PROCESS_CLOCKS', '-D_WASI_EMULATED_GETPID']
c_link_args = ['-lwasi-emulated-signal', '-lwasi-emulated-mman', '-lwasi-emulated-process-clocks', '-lwasi-emulated-getpid']
''')

ffi = source/'libffi-3.5.2'
ffibuild = cache/'libffi-build'
ffibuild.mkdir(exist_ok=True)
ffienv = dict(env, CC=str(cc), AR=str(ar), RANLIB=str(sdk/'bin/llvm-ranlib'))
if not (ffibuild/'include/ffi.h').exists():
    run(ffi/'configure','--host=wasm32-wasi','--disable-shared','--enable-static',f'--prefix={prefix}',cwd=ffibuild,extra_env=ffienv)
objects=[]
for file in [ffi/'src/prep_cif.c',ffi/'src/types.c',NATIVE/'ffi-wasi.c']:
    obj=cache/(file.stem+'.o')
    run(cc,'-O2','-DHAVE_CONFIG_H','-I',ffibuild,'-I',ffibuild/'include','-I',ffi/'include','-c',file,'-o',obj)
    objects.append(obj)
run(ar,'rcs',prefix/'lib/libffi.a',*objects)
for name in ['ffi.h','ffitarget.h']:
    shutil.copyfile(ffibuild/'include'/name,prefix/'include'/name)
(prefix/'lib/pkgconfig/libffi.pc').write_text(f'''prefix={prefix}
libdir=${{prefix}}/lib
includedir=${{prefix}}/include
Name: libffi
Description: WASI scalar libffi
Version: 3.5.2
Libs: -L${{libdir}} -lffi
Cflags: -I${{includedir}}
''')
pcrebuild=cache/'pcre2-build'
run('cmake','-S',source/'pcre2-10.44','-B',pcrebuild,f'-DCMAKE_TOOLCHAIN_FILE={sdk}/share/cmake/wasi-sdk.cmake',f'-DCMAKE_INSTALL_PREFIX={prefix}','-DBUILD_SHARED_LIBS=OFF','-DPCRE2_BUILD_TESTS=OFF','-DPCRE2_BUILD_PCRE2GREP=OFF','-DPCRE2_SUPPORT_JIT=OFF','-DPCRE2_NEWLINE=ANY')
run('cmake','--build',pcrebuild,'--parallel','8')
run('cmake','--install',pcrebuild)

glib=source/'glib-2.86.1'
if 'stellar_type_init' not in (glib/'gobject/gtype.c').read_text():
    run('patch','-p1','--input',HERE/'glib-wasi.patch',cwd=glib)
build=cache/'glib-build'
options=['--prefix',prefix,'--cross-file',cross,'--default-library','static','-Dtests=false','-Dinstalled_tests=false','-Ddocumentation=false','-Dintrospection=disabled','-Dnls=disabled','-Dselinux=disabled','-Dlibmount=disabled','-Dxattr=false','-Dsysprof=disabled','-Dlibelf=disabled']
run(meson,'setup',*(['--reconfigure'] if (build/'build.ninja').exists() else []),build,glib,*options)
run('ninja','-C',build,'glib/libglib-2.0.a','gobject/libgobject-2.0.a','gio/libgio-2.0.a','subprojects/proxy-libintl/libintl.a')
output=cache/'glib.wasm'
flags=['-D_WASI_EMULATED_'+x for x in ['SIGNAL','MMAN','PROCESS_CLOCKS','GETPID']]
includes=[source/'glib-2.86.1',source/'glib-2.86.1/glib',source/'glib-2.86.1/gmodule',build,build/'glib']
libraries=[build/'gio/libgio-2.0.a',build/'gobject/libgobject-2.0.a',build/'glib/libglib-2.0.a',build/'subprojects/proxy-libintl/libintl.a',prefix/'lib/libffi.a',prefix/'lib/libpcre2-8.a',prefix/'lib/libz.a']
run(wrapper,'-O2','-Wl,--export-table','-Wl,--strip-debug',*flags,*['-I'+str(x) for x in includes],HERE/'glib-fixture.c',*libraries,*['-lwasi-emulated-'+x for x in ['signal','mman','process-clocks','getpid']],'-o',output)
target=args.output or NATIVE.parent/'wasmffi/testdata/glib.wasm'
if args.check:
    if output.read_bytes()!=target.read_bytes():
        raise SystemExit('GLib fixture drift')
else:
    shutil.copyfile(output,target)
    target.chmod(0o644)
print('GLib fixture verified' if args.check else 'GLib fixture rebuilt')
