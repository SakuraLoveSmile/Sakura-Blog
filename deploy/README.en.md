# Deployment guide

The production artifact is a single Go binary. The Console and User Center assets are built by Vite and embedded into that binary. `update.sh` verifies the downloaded binary with SHA256 before deployment.

`update.sh` requires `curl`, `sha256sum`, `systemctl`, and `python3`. It supports public and private GitHub repositories; a private repository uses a `contents:read` PAT in `/opt/blog/.token` to read Release assets.

Every new Release must include `blog-linux-amd64`, `blog-linux-amd64.sha256`, and `blog-linux-amd64.spdx.json`. The updater downloads the binary and checksum, verifies the checksum, and only then performs the replacement, health check, and automatic rollback flow. The SBOM remains available for release review and dependency analysis.

## Configuration

Create the runtime configuration outside Git:

```bash
mkdir -p /opt/blog/{config,data,content/posts,update}
chown -R blog:blog /opt/blog
cp config/app.example.yml /opt/blog/config/app.yml
cp config/site.example.yml /opt/blog/config/site.yml
chmod 600 /opt/blog/config/app.yml /opt/blog/config/site.yml
```

Prefer environment injection for tokens and passwords. Do not commit `config/app.yml`, `config/site.yml`, or a systemd `EnvironmentFile` containing secrets.

## systemd and nginx

The files in this directory are examples:

- `blog.service` runs the binary as the unprivileged `blog` user (`ReadWritePaths` includes `/opt/blog/data`).
- `blog.service.d/10-secrets.conf.example` shows how to load secrets from a root-owned environment file.
- `nginx.conf.example` terminates TLS and forwards requests to the local service.
- `blog-update.service` is a manual update entry point (`sudo systemctl start blog-update`, equivalent to `sudo update.sh`). Console update triggers were removed in v1.7.3; the Console only checks for new versions.

Copy and review each file for the target host before enabling it. Set `BLOG_TRUSTED_PROXY` or the matching YAML setting only to the actual reverse-proxy address range.

## Build and health check

From the repository worktree:

```bash
pnpm install --frozen-lockfile
make build
./blog -root /opt/blog
```

Run `bash deploy/update_test.sh` to exercise update, request consumption, and rollback logic without contacting GitHub or systemd.
