# AGENTS.md

## Cursor Cloud specific instructions

### Overview

This is `mattermost-plugin-meet`, a Mattermost plugin for Google Meet integration. It is **not** a standalone application — it compiles into a `.tar.gz` bundle that is deployed to a running Mattermost server. There are no services to start locally.

### Components

- **Server** (`server/`): Go plugin binary. Built with `go build` for multiple architectures.
- **Webapp** (`webapp/`): React/TypeScript frontend bundle built with Webpack.

### Required toolchain

- **Go 1.25+** — must be installed at `/usr/local/go` (the update script handles this).
- **Node.js v24.13.1** — specified in `.nvmrc`. The update script runs `nvm install`.
- **npm** — used for webapp dependencies (`cd webapp && npm install`).

### Key commands (all via `Makefile`)

| Command | Purpose |
|---|---|
| `make test` | Run Go tests (gotestsum) + JS tests (jest) |
| `make check-style` | ESLint + golangci-lint |
| `make dist` | Full build: server (all archs) + webapp + tar.gz bundle |
| `make deploy` | Build + deploy to a Mattermost server (needs env vars) |
| `make install-go-tools` | Install golangci-lint and gotestsum into `./bin/` |

### Gotchas

- `make check-style` has pre-existing ESLint errors in `webapp/src/index.tsx` (import order, prop validation, etc.). These are upstream issues, not environment problems.
- The `fatal: No names found, cannot describe anything.` git warning during builds is expected — the repo has no version tags, so the version defaults to `0.0.0+<hash>`.
- Go 1.25 is required (`go.mod`). The system Go shipped with the VM may be older; the update script installs Go 1.25.x from `go.dev`.
- `build/setup.mk` compiles helper tools (`build/bin/manifest`, `build/bin/pluginctl`) on first `make` invocation — this is normal.
- The `GOBIN` env var is set to `$PWD/bin` by the Makefile, so Go tools like `golangci-lint` and `gotestsum` are installed locally in `./bin/`.
