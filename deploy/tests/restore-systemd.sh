#!/usr/bin/env bash
# 只在独立 Docker 容器中验证，绝不修改宿主机 systemd 或生产数据。
set -euo pipefail
project_root=$(cd "$(dirname "$0")/../.." && pwd)
test_root=$(mktemp -d)
container_name="blog-phase-b-systemd-$$"
cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  rm -rf "$test_root"
}
trap cleanup EXIT
architecture=$(docker info --format '{{.Architecture}}')
case "$architecture" in aarch64|arm64) architecture=arm64;; x86_64|amd64) architecture=amd64;; *) exit 1;; esac
cd "$project_root"
for package in maintenance backup store app; do
  CGO_ENABLED=0 GOOS=linux GOARCH="$architecture" go test -c "./internal/$package" -o "$test_root/$package.test"
done
cp deploy/blog.service "$test_root/blog.service"
cp deploy/update.sh "$test_root/update.sh"
cp deploy/blog-update.service "$test_root/"
printf 'exit 0\n' > "$test_root/lock-noop.sh"
chmod 755 "$test_root"
cp deploy/tests/restore-permissions.py "$test_root/permissions.py"
docker build -t blog-phase-b-systemd-test -f deploy/tests/restore-systemd.Dockerfile deploy/tests >/dev/null
docker run -d --name "$container_name" --privileged --cgroupns=private --tmpfs /run --tmpfs /run/lock --tmpfs /tmp --mount "type=bind,source=$test_root,target=/checks,readonly" blog-phase-b-systemd-test >/dev/null
for attempt in $(seq 1 30); do
  if docker exec "$container_name" systemctl list-units >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$container_name" bash -c 'useradd -r blog; mkdir -p /opt/blog/config /opt/blog/data /opt/blog/update; cp /bin/true /opt/blog/blog; touch /opt/blog/config/app.yml /opt/blog/config/site.yml; chown blog:blog /opt/blog/config/app.yml /opt/blog/config/site.yml /opt/blog/data; chmod 600 /opt/blog/config/app.yml /opt/blog/config/site.yml; systemd-analyze verify /checks/blog.service'
docker exec "$container_name" bash -c 'source /checks/update.sh; run_with_maintenance_lock' /checks/lock-noop.sh
docker exec "$container_name" systemd-run --wait --collect --pipe --unit blog-existing-lock -p User=blog -p NoNewPrivileges=true -p ProtectSystem=strict -p ReadWritePaths=/opt/blog/data -p Environment=BLOG_TEST_OPERATION_LOCK=/opt/blog/data/maintenance.lock /checks/maintenance.test -test.run TestServiceUserCanTakeExistingOperationLock -test.v
docker exec "$container_name" systemd-run --wait --collect --pipe --unit blog-restore-permissions -p User=blog -p NoNewPrivileges=true -p ProtectSystem=strict -p 'ReadWritePaths=/opt/blog/data /opt/blog/config/site.yml' /usr/bin/python3 /checks/permissions.py
for package in maintenance backup store app; do
  docker exec "$container_name" "/checks/$package.test" -test.v
 done
