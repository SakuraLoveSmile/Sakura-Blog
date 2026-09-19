"""只在隔离的 systemd 容器中验证 path 单实例等待及失败退避。"""
import fcntl
import os
from pathlib import Path
import shutil
import subprocess
import time

root = Path('/opt/blog')
request = root / 'update/requests/lock-test.json'
done = root / 'update-done'
for name in ['blog-update.service', 'blog-update.path']:
    shutil.copyfile('/checks/' + name, '/etc/systemd/system/' + name)
runner = root / 'update.sh'
runner.write_text('''#!/bin/bash
set -eu
if [[ "${BLOG_MAINTENANCE_LOCK_HELD:-}" != "1" ]]; then
  source /checks/update.sh
  run_with_maintenance_lock
else
  rm /opt/blog/update/requests/lock-test.json
  touch /opt/blog/update-done
fi
''')
runner.chmod(0o755)
def systemctl(*args):
    try:
        return subprocess.check_output(['systemctl', *args], text=True).strip()
    except subprocess.CalledProcessError:
        subprocess.run(['journalctl', '-u', 'blog-update.service', '-u', 'blog-update.path', '--no-pager', '-n', '35'])
        subprocess.run(['systemctl', 'status', 'blog-update.service', '--no-pager'])
        raise
def wait_until(check):
    end = time.monotonic() + 15
    while time.monotonic() < end:
        if check(): return
        time.sleep(.1)
    raise AssertionError('systemd state timeout')
systemctl('daemon-reload')
# root 创建的锁需允许 blog 用户打开。
lock = root / 'data/maintenance.lock'
with lock.open('a+') as held:
    fcntl.flock(held, fcntl.LOCK_EX)
    request.write_text('{}')
    systemctl('start', 'blog-update.path')
    wait_until(lambda: systemctl('show', 'blog-update.service', '-p', 'MainPID', '--value') != '0')
    pid = systemctl('show', 'blog-update.service', '-p', 'MainPID', '--value')
    time.sleep(1)
    assert request.exists() and not done.exists()
    assert systemctl('show', 'blog-update.service', '-p', 'MainPID', '--value') == pid
    assert systemctl('show', 'blog-update.service', '-p', 'NRestarts', '--value') == '0'
    fcntl.flock(held, fcntl.LOCK_UN)
    wait_until(done.exists)
    assert not request.exists()
systemctl('stop', 'blog-update.path', 'blog-update.service')
done.unlink()
# 不可恢复日志不允许更新，失败服务等待 RestartSec，不被 path 紧密重新拉起。
backups = root / 'data/backups'
backups.mkdir(exist_ok=True)
journal = backups / '.restore-blocked.json'
journal.write_text('{"version":2,"status":"succeeded"}')
request.write_text('{}')
systemctl('start', 'blog-update.path')
wait_until(lambda: systemctl('show', 'blog-update.service', '-p', 'SubState', '--value') == 'auto-restart')
count = systemctl('show', 'blog-update.service', '-p', 'NRestarts', '--value')
time.sleep(1)
assert request.exists() and not done.exists()
assert systemctl('show', 'blog-update.service', '-p', 'NRestarts', '--value') == count
systemctl('stop', 'blog-update.path', 'blog-update.service')
journal.unlink()
request.unlink()
# 已消费请求后的普通更新失败不得自动重新部署同一个坏版本。
runner.write_text(runner.read_text().replace('  touch /opt/blog/update-done', '  touch /opt/blog/update-done\n  exit 1'))
request.write_text('{}')
systemctl('start', 'blog-update.path')
wait_until(lambda: systemctl('show', 'blog-update.service', '-p', 'ActiveState', '--value') == 'failed')
assert done.exists() and not request.exists()
assert systemctl('show', 'blog-update.service', '-p', 'NRestarts', '--value') == '0'
assert systemctl('show', 'blog-update.service', '-p', 'SubState', '--value') == 'failed'
systemctl('stop', 'blog-update.path', 'blog-update.service')
print('systemd: same-process wait, resume, exit75 backoff and no exit1 retry passed')
