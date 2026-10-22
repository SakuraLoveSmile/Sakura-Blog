"""Record exact Cargo source checksums and copy the corresponding notices."""
import hashlib
import json
import os
from pathlib import Path
import tarfile
import tomllib

HERE = Path(__file__).resolve().parent
cargo = Path(os.environ.get('CARGO_HOME', str(Path.home() / '.cargo')))
output = HERE / 'licenses'
output.mkdir(exist_ok=True)
rows = []
packages = tomllib.loads((HERE / 'Cargo.lock').read_text())['package']
ports = json.loads((HERE / 'rust-ports.json').read_text())
for port in ports:
    packages.append({'name': port['name'], 'version': port['version'], 'checksum': port['sha256'],
                     'source': 'registry+https://github.com/rust-lang/crates.io-index'})
for package in sorted(packages, key=lambda p: (p['name'], p['version'])):
    if not package.get('source', '').startswith('registry+'):
        continue
    name = package['name'] + '-' + package['version']
    archives = list((cargo / 'registry/cache').glob('*/' + name + '.crate'))
    if not archives:
        raise SystemExit('crate is not cached: ' + name)
    archive = archives[0]
    if hashlib.sha256(archive.read_bytes()).hexdigest() != package['checksum']:
        raise SystemExit('crate checksum mismatch: ' + name)
    notices = []
    with tarfile.open(archive) as tar:
        manifest = tomllib.loads(tar.extractfile(name + '/Cargo.toml').read().decode())['package']
        for member in tar.getmembers():
            path = Path(member.name)
            if not member.isfile() or len(path.parts) < 2:
                continue
            if len(path.parts) == 2 or path.parts[1].lower() in {'licenses', 'licences'}:
                if path.name.lower().startswith(('license', 'licence', 'copying', 'copyright', 'unlicense')):
                    target = output / (name + '-' + path.name)
                    data = tar.extractfile(member).read()
                    if target.exists() and target.read_bytes() != data:
                        raise SystemExit('conflicting notice: ' + str(target))
                    target.write_bytes(data)
                    notices.append('licenses/' + target.name)
    rows.append({'name': package['name'], 'version': package['version'],
                 'license': manifest.get('license'), 'sha256': package['checksum'],
                 'source': 'https://static.crates.io/crates/' + package['name'] + '/' + name + '.crate',
                 'notices': sorted(set(notices)),
                 **({'notice_status': 'upstream crate contains no license text'} if not notices else {})})
(HERE / 'dependency-licenses.json').write_text(json.dumps(rows, indent=2) + '\n')
print('Recorded', len(rows), 'checksum-verified crate notices')
