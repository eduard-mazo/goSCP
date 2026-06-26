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
- 🔒 Bearer-token auth (with optional password login); all file ops confined to one root dir (path-traversal safe)
- 📦 Single binary — the Vue SPA is `go:embed`-ed, works fully offline
- 🖥️ Cross-compiles to Linux, Windows, Android/Termux & Advantech ICR routers — zero CGO
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

## Authentication

The API is guarded by a **bearer token**. By default a random one is generated
at startup and printed to the console — paste it into the login screen, or pin a
fixed value with `--token` / `GOSCP_TOKEN`.

For headless deployments where reading the console is awkward (e.g. a router),
enable **password login** with `--password` / `GOSCP_PASSWORD`. Clients then
exchange the password for the bearer token via `POST /api/v1/token`:

```bash
./bin/goscp --root /srv/share --password s3cret
# fetch a token using the password:
curl -s -X POST localhost:8080/api/v1/token \
  -H 'Content-Type: application/json' -d '{"password":"s3cret"}'
# → {"token":"3f9a…"}
```

When `--password` is set the web login screen defaults to a password field (with
"Use an access token instead" as a fallback). Password login is **off** unless
`--password` / `GOSCP_PASSWORD` is provided; `POST /api/v1/token` is the only
public route — every other `/api` endpoint still requires the bearer token.

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
make build-icr       # Advantech ICR arm/v7  → bin/goscp-icr-armv7
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

To share the phone's actual storage (Downloads, DCIM, …), run `termux-setup-storage`
once and point `--root` at the `~/storage` directory it creates:

```bash
# in Termux
termux-setup-storage              # creates ~/storage with links into shared storage
./goscp-android-arm64 --root ~/storage --port 8080
```

`~/storage` is a directory of symlinks (e.g. `downloads -> /storage/emulated/0/Download`).
goSCP follows symlinked entries when listing, so these linked folders show up as
directories you can browse into — not as opaque files.

> 32-bit (armv7) Termux is not built by default: PIE on 32-bit ARM needs the
> Android NDK C toolchain. It's a rare configuration; reach for arm64.

### Advantech ICR-323x (industrial router)

The ICR-323x runs a 32-bit ARM (`arm/v7`) BusyBox userland with no package
manager and an older glibc. Like the Android target, goSCP cross-builds a
**fully static** binary (`CGO_ENABLED=0`, no ELF interpreter, no libc dep) that
runs as-is on the device:

```bash
make build-icr        # → bin/goscp-icr-armv7 (static arm/v7, frontend embedded)
make verify-arm       # assert it really is a static ARM ELF before shipping
```

`make deploy-icr` builds the binary and **prints** copy-paste `scp`/`ssh` steps —
it never pushes anything itself (no credentials in the build). Override the
device IP with `DEVICE=`:

```bash
make deploy-icr DEVICE=192.168.1.1
```

The device's `/root` persists across reboots via OverlayFS, so the printed steps
place the binary, served data and logs under `/root/bin`, `/root/data`,
`/root/log`. To run goSCP as a service that survives reboots, generate a BusyBox
`init.d` script:

```bash
make service-icr      # writes bin/goscp.init + prints install steps
```

Edit the `GOSCP_TOKEN` / `GOSCP_PASSWORD` lines at the top of `bin/goscp.init` to
pin credentials (otherwise a random token is generated on every boot), then
install it as printed:

```bash
scp bin/goscp.init root@192.168.1.1:/etc/init.d/goscp
ssh root@192.168.1.1 'chmod +x /etc/init.d/goscp && /etc/init.d/goscp start'
```

## Configuration

| Flag             | Env var               | Default | Description                                   |
|------------------|-----------------------|---------|-----------------------------------------------|
| `--port`         | `GOSCP_PORT`          | `8080`  | TCP port to listen on                         |
| `--host`         | `GOSCP_HOST`          | (all)   | Interface to bind (empty = all interfaces)    |
| `--addr`         | `GOSCP_ADDR`          | (unset) | Full listen address; overrides `--host`/`--port` |
| `--root`         | `GOSCP_ROOT`          | `.`     | Directory exposed for file exchange           |
| `--token`        | `GOSCP_TOKEN`         | random  | Bearer token (auto-generated if unset)        |
| `--password`     | `GOSCP_PASSWORD`      | (unset) | Enable `POST /api/v1/token`: exchange this password for the token |
| `--max-upload-mb`| `GOSCP_MAX_UPLOAD_MB` | `2048`  | Max upload size per request (MB)              |

## API

All endpoints live under `/api/v1` and require `Authorization: Bearer <token>`,
except `POST /token`, which is public (it's how a client obtains the token).

| Method   | Path          | Description                          |
|----------|---------------|--------------------------------------|
| `POST`   | `/token`      | Exchange `{ "password" }` for the bearer token (public; needs `--password`) |
| `GET`    | `/health`     | Liveness check                       |
| `GET`    | `/usage`      | Root statistics (counts, total size) |
| `GET`    | `/dirsize?path=` | Recursive size + counts for one directory |
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
