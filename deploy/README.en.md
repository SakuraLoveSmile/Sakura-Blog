# Deployment guide

The production artifact is a Docker image. `.github/workflows/docker-release.yml` builds and pushes `ghcr.io/sakuralovesmile/sakura-blog` on tag pushes; servers run it through Docker Compose with a pinned version tag. The repository no longer builds or ships a standalone binary, and the host-binary `update.sh` / `update-docker.sh` scripts and systemd update units have been removed.

To migrate a 1.x installation to Stellar 2.0, follow [Docker / Compose migration](upgrade-v1-to-v2.md) first: preview against the old database, rehearse in isolation, then migrate during a maintenance window. Regular image upgrades and `-seed` are not a substitute for the data migration.

## Configuration

Create the runtime configuration outside Git:

```bash
mkdir -p /opt/1panel/docker/compose/blog/{config,data,content/posts}
cp config/app.example.yml  /opt/1panel/docker/compose/blog/config/app.yml
cp config/site.example.yml /opt/1panel/docker/compose/blog/config/site.yml
chmod 600 /opt/1panel/docker/compose/blog/config/app.yml /opt/1panel/docker/compose/blog/config/site.yml
```

Prefer environment injection for tokens and passwords. Do not commit `config/app.yml`, `config/site.yml`, or an environment file containing secrets.

## Docker Compose

`deploy/docker-compose.image.yml` is a reference compose file. It mounts only runtime subdirectories (`config/`, `data/`, `content/`) and must pin a concrete `X.Y.Z` image tag in production; `latest` is not valid version evidence.

```bash
docker compose pull
docker compose up -d
curl -fsS http://127.0.0.1:8081/readyz
```

Upgrade and rollback both work by changing the pinned `image` tag and running `docker compose up -d`. If the target version includes data migrations, restore the pre-upgrade backup before switching back.

## nginx, Console and operations

- `nginx.conf.example` terminates TLS and forwards requests to the container's host port.
- The Console "System Update" page (`/console/system-update`) reads public GHCR image tags and only reports whether a newer version exists; it does not trigger upgrades. Release notes, publish time and the "release notes" link come from the notes-only GitHub Release created for the tag; when it is missing or mismatched, only version information is shown.
- Automatic full backups are written to `data/backups/` every 24h (database, uploads and site.yml), retention is controlled by `backup_keep`.
- Logs: `docker logs -f blog`.
- Site restore runs inside the application; keep `data/` writable and `config/site.yml` as a single writable file, and do not delete unfinished restore journals.

See `deploy/README.md` (Chinese) for the full deployment checklist, upgrade/rollback and restore guidance.
