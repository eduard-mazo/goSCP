# goSCP

A self-contained, **WinSCP-style file-exchange server** that ships as a **single
offline binary**. Files are exchanged over HTTP through a polished web UI
(Vue 3 + shadcn-vue + Tailwind) that is **embedded directly into the Go
executable** — no external assets, no CDN, no runtime dependencies.

```
┌──────────────────────────────────────────────┐
│  goSCP dev                                     │
│  HTTP file exchange — single offline binary    │
├──────────────────────────────────────────────┤
│  Listening : http://localhost:8080             │
│  Root dir  : /srv/share                        │
│  Token     : 3f9a…                             │
└──────────────────────────────────────────────┘
```

## Features

- 📁 Browse, upload (incl. drag-and-drop), download, rename, delete, mkdir
- 🔒 Bearer-token auth; all file ops confined to one root dir (path-traversal safe)
- 📦 Single binary — the Vue SPA is `go:embed`-ed, works fully offline
- 🖥️ Cross-compiles to Linux & Windows with zero CGO
- ⚡ Stdlib-only backend (Go 1.22+ method routing) — no third-party Go deps

## Quick start

```bash
make deps      # install Go modules + npm packages
make run       # build frontend + binary, then run (serves ./ )
```

Open <http://localhost:8080> and paste the token printed in the console.

Serve a specific directory:

```bash
make run ROOT=/srv/share
# or run the binary directly:
./bin/goscp --root /srv/share --port 8080 --token mysecret
```

## Development

Two terminals for hot-reload frontend against the live API:

```bash
make dev        # terminal 1: Go API on :8080 (token = devtoken)
make web-dev    # terminal 2: Vite dev server on :5173 (proxies /api → :8080)
```

## Build & release

```bash
make build           # host binary           → bin/goscp
make build-linux     # Linux amd64           → bin/goscp-linux-amd64
make build-windows   # Windows amd64         → bin/goscp-windows-amd64.exe
make build-android   # Android/Termux arm64  → bin/goscp-android-arm64
make release         # all of the above
```

Each binary embeds the freshly built frontend, so a release artifact is the only
file you need to deploy.

### Android (Termux)

The Android target is built as a static, **position-independent** arm64 binary
(`-buildmode=pie`, `CGO_ENABLED=0`) — modern Android only executes PIE binaries,
and arm64 covers virtually all current devices.

Copy `bin/goscp-android-arm64` into Termux and run it:

```bash
# in Termux
mkdir -p ~/share
chmod +x goscp-android-arm64
./goscp-android-arm64 --root ~/share --port 8080
```

Then open `http://localhost:8080` in the phone's browser (or reach it from
another device on the same Wi-Fi via the phone's LAN IP and `--host 0.0.0.0`).
No `pkg install` is needed — the binary is self-contained.

> 32-bit (armv7) Termux is not built by default: PIE on 32-bit ARM needs the
> Android NDK C toolchain. It's a rare configuration; reach for arm64.

## Configuration

| Flag             | Env var               | Default | Description                                   |
|------------------|-----------------------|---------|-----------------------------------------------|
| `--port`         | `GOSCP_PORT`          | `8080`  | TCP port to listen on                         |
| `--host`         | `GOSCP_HOST`          | (all)   | Interface to bind (empty = all interfaces)    |
| `--addr`         | `GOSCP_ADDR`          | (unset) | Full listen address; overrides `--host`/`--port` |
| `--root`         | `GOSCP_ROOT`          | `.`     | Directory exposed for file exchange           |
| `--token`        | `GOSCP_TOKEN`         | random  | Bearer token (auto-generated if unset)        |
| `--max-upload-mb`| `GOSCP_MAX_UPLOAD_MB` | `2048`  | Max upload size per request (MB)              |

## API

All endpoints require `Authorization: Bearer <token>` and live under `/api/v1`.

| Method   | Path          | Description                          |
|----------|---------------|--------------------------------------|
| `GET`    | `/health`     | Liveness check                       |
| `GET`    | `/usage`      | Root statistics (counts, total size) |
| `GET`    | `/files?path=`| List a directory                     |
| `GET`    | `/download?path=` | Stream a file                    |
| `POST`   | `/upload`     | Multipart upload (`path`, `files[]`) |
| `POST`   | `/mkdir`      | `{ "path", "name" }`                 |
| `POST`   | `/rename`     | `{ "path", "name" }`                 |
| `DELETE` | `/files?path=`| Delete a file/folder                 |

## Project layout

```
cmd/goscp/            # main: flags, signals, graceful shutdown
internal/config/      # configuration (flags + env)
internal/storage/     # path-confined filesystem operations
internal/api/         # JSON handlers, auth/CORS/logging middleware
internal/server/      # HTTP server, SPA fallback, wiring
internal/assets/      # go:embed of the built frontend (dist/)
web/                  # Vue 3 + Vite + shadcn-vue + Tailwind SPA
Makefile              # build / run / cross-compile / lint
```
# goSCP
