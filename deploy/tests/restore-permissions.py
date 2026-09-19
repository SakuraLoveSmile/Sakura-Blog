import errno
import os

with open('/proc/self/status') as source:
    assert 'NoNewPrivs:\t1' in source.read(), 'NoNewPrivileges is not active'
with open('/opt/blog/config/site.yml', 'r+') as target:
    target.seek(0)
    target.write('title: restored\n')
    target.truncate()
    target.flush()
    os.fsync(target.fileno())
for path in ['/opt/blog/config/app.yml', '/opt/blog/config/unexpected.yml', '/opt/blog/unexpected']:
    try:
        with open(path, 'w') as target:
            target.write('must not be writable')
    except OSError as error:
        assert error.errno == errno.EROFS, (path, error)
    else:
        raise AssertionError('ProtectSystem allowed write outside site.yml/data: ' + path)
print('PASS: NoNewPrivileges and ProtectSystem=strict; only existing site.yml/data are writable')
