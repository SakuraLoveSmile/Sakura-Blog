"""更新/恢复共享锁回归；只在临时目录运行，不访问网络或真实服务。"""
import fcntl
import json
import os
from pathlib import Path
import select
import subprocess
import tempfile

script = Path(__file__).resolve().parents[1] / 'update.sh'
with tempfile.TemporaryDirectory(prefix='blog-update-lock-') as tmp:
    root = Path(tmp)
    data = root / 'data'
    data.mkdir()
    requests = root / 'update' / 'requests'
    requests.mkdir(parents=True)
    request = requests / 'test.json'
    done = root / 'done'
    child = root / 'child.sh'
    child.write_text('set -eu\nrm "$INSTALL_DIR/update/requests/test.json"\ntouch "$INSTALL_DIR/done"\n')
    env = dict(os.environ, INSTALL_DIR=str(root), BLOG_DB=str(data / 'blog.db'),
               BLOG_MAINTENANCE_LOCK=str(data / 'maintenance.lock'),
               BLOG_LOCK_FILE=str(root / 'update' / 'update.lock'))
    command = ['bash', '-c', 'source "$1"; run_with_maintenance_lock', str(child), str(script)]
    for lock in [Path(env['BLOG_MAINTENANCE_LOCK']), Path(env['BLOG_LOCK_FILE'])]:
        request.write_text('{}')
        with lock.open('a+') as handle:
            fcntl.flock(handle, fcntl.LOCK_EX)
            proc = subprocess.Popen(command, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                assert select.select([proc.stderr], [], [], 10)[0], 'no lock-wait acknowledgement'
                assert '等待' in proc.stderr.readline().decode(), 'unexpected lock response'
                assert proc.poll() is None and request.exists() and not done.exists()
                fcntl.flock(handle, fcntl.LOCK_UN)
                _, errors = proc.communicate(timeout=10)
                assert proc.returncode == 0, errors
                assert done.exists() and not request.exists()
            finally:
                if proc.poll() is None:
                    proc.kill()
                    proc.wait()
        done.unlink()
    journals = data / 'backups'
    journals.mkdir()
    journal = journals / '.restore-test.json'
    for payload in [{'version': 1, 'status': 'running'}, {'version': 2, 'status': 'succeeded'}, []]:
        journal.write_text(json.dumps(payload))
        request.write_text('{}')
        result = subprocess.run(command, env=env, capture_output=True, timeout=10)
        assert result.returncode == 75 and request.exists() and not done.exists()
    journal.unlink()
    # root 执行器不能跟随服务用户可写目录中的符号链接。
    lock = Path(env['BLOG_MAINTENANCE_LOCK'])
    lock.unlink()
    victim = root / 'victim'
    victim.write_text('untouched')
    lock.symlink_to(victim)
    result = subprocess.run(command, env=env, capture_output=True, timeout=10)
    assert result.returncode == 75 and victim.read_text() == 'untouched' and request.exists()
print('update-lock: wait, resume, journal rejection and symlink checks passed')
