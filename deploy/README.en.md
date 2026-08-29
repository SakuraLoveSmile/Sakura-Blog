# Deployment guide

The production artifact is a single Go binary. The Console and User Center assets are built by Vite and embedded into that binary. `update.sh` verifies both SHA256 integrity and GitHub Artifact Attestation provenance.

`update.sh` requires `curl`, `sha256sum`, `systemctl`, `python3`, and a GitHub CLI that supports `gh attestation verify --bundle`. Provenance verification is strict by default; temporarily set `BLOG_VERIFY_PROVENANCE=0` only while migrating releases created before attestation bundles were published.

Public repositories support anonymous updates and do not require a PAT. The binary, SHA256 file, and `blog-linux-amd64.attestation.jsonl` are public Release assets; the updater verifies the downloaded binary against that bundle while constraining the repository and release-workflow signer to `SakuraLoveSmile/Sakura-Blog`. A private repository may still use a `contents:read` PAT in `/opt/blog/.token` to read Release assets; anonymous bundle verification does not depend on a token.

Every new Release must include `blog-linux-amd64`, its `.sha256` file, and `blog-linux-amd64.attestation.jsonl`. Releases without a bundle are rejected by default. For a one-time migration only, explicitly set `BLOG_VERIFY_PROVENANCE=0` (for example, `sudo BLOG_VERIFY_PROVENANCE=0 /opt/blog/update.sh`), then restore the strict default immediately.

## Configuration

Create the runtime configuration outside Git:

```bash
mkdir -p /opt/blog/config /opt/blog/data /opt/blog/content/posts
cp config/app.example.yml /opt/blog/config/app.yml
cp config/site.example.yml /opt/blog/config/site.yml
chmod 600 /opt/blog/config/app.yml /opt/blog/config/site.yml
```

Prefer environment injection for tokens and passwords. Do not commit `config/app.yml`, `config/site.yml`, or a systemd `EnvironmentFile` containing secrets.

## systemd and nginx

The files in this directory are examples:

- `blog.service` runs the binary as the unprivileged `blog` user.
- `blog.service.d/10-secrets.conf.example` shows how to load secrets from a root-owned environment file.
- `nginx.conf.example` terminates TLS and forwards requests to the local service.
- `blog-update.timer` and `update.sh` provide an optional release update flow with rollback.

Copy and review each file for the target host before enabling it. Set `BLOG_TRUSTED_PROXY` or the matching YAML setting only to the actual reverse-proxy address range.

## Build and health check

From the repository worktree:

```bash
pnpm install --frozen-lockfile
make build
./blog -root /opt/blog
```

Run `bash deploy/update_test.sh` to exercise update and rollback logic without contacting GitHub or systemd.
