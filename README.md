# Kutyaguru

Desktop app that converts Booked4us Excel exports into Számlázz.hu-compatible
CSV for a Hungarian dog-training business.

Built with **Wails v3** (Go backend, React + Mantine frontend).

## Prerequisites

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

## Live development

```sh
wails3 dev
```

Runs a Vite dev server with hot reload for the frontend while serving the Go
services. The dev server port can be overridden with `WAILS_VITE_PORT`
(defaults to 9245).

## Building

```sh
wails3 build
```

Produces the production binary at `bin/kutyaguru-booking`.

### Cross-building the Windows `.exe` from macOS

Wails v3 Windows builds are pure Go (CGO disabled; WebView2 is loaded at runtime
via a pure-Go loader), so the Windows binary can be produced on a macOS host with
only the Go toolchain, the `wails3` CLI, and Node/npm — no mingw-w64, Zig, or
Docker. The `Makefile` wraps this:

```sh
make windows-setup        # one-time: install the wails3 CLI + frontend deps
make windows              # cross-build bin/kutyaguru-booking.exe (amd64)
make windows ARCH=arm64   # cross-build for Windows on ARM
make help                 # list all targets
```

Building the NSIS installer (`make windows-package`) still requires a Windows
host with `makensis`.

## Layout

- `main.go` — app entry point: creates the window, wires geometry persistence.
- `booking.go` — the `Booking` service (Excel parsing, field mapping, CSV export).
- `assets/` — embedded `fields.yaml` and CSV template.
- `frontend/src/` — React UI (Mantine v9, react-data-grid).
- `frontend/bindings/` — generated TypeScript bindings (`wails3 generate bindings`).
- `Taskfile.yml` / `build/` — Wails v3 build configuration.

Settings (including window geometry) persist to
`<UserConfigDir>/kutyaguru/settings.json`.

## License

MIT — see [LICENSE](LICENSE). Third-party dependency attributions are listed
in [NOTICE](NOTICE).
