# Stellar Blog

Stellar Blog is a self-hosted blog built with Go, Gin, server-rendered HTML, and a Vue 3 Console. It includes a separate User Center at `/uc/`, SQLite persistence, a dark-only Console design, and a compile-time module system for extension points.

## Quick start

Requirements: Go 1.25.5 or newer, Node.js 20 or newer, and pnpm 8.15.4.

```bash
cp config/app.example.yml config/app.yml
cp config/site.example.yml config/site.yml
pnpm install --frozen-lockfile
make build
./blog -seed
```

The default development server listens on `http://localhost:8080`. Use `-root` when starting the binary from another working directory.

## Project layout

- `cmd/blog/` — application composition root
- `internal/` — shared services, storage, identity, and persistence
- `modules/console/` — Console API and Vue applications (`/console/` and `/uc/`)
- `modules/stellar/` — server-rendered public site
- `packages/` — reusable API client, components, editor, and shared utilities
- `deploy/` — systemd, nginx, and update examples

## Development

All changes are made on a feature branch and must pass:

```bash
make test
make build
```

See [deploy/README.en.md](deploy/README.en.md) for production deployment and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for licensing information.

## License

The project is distributed under GNU GPLv3 or later. See [LICENSE](LICENSE) and [docs/licensing.md](docs/licensing.md).

