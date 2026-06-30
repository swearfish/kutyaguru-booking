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

## Layout

- `main.go` — app entry point: creates the window, wires geometry persistence.
- `booking.go` — the `Booking` service (Excel parsing, field mapping, CSV export).
- `assets/` — embedded `fields.yaml` and CSV template.
- `frontend/src/` — React UI (Mantine v9, react-data-grid).
- `frontend/bindings/` — generated TypeScript bindings (`wails3 generate bindings`).
- `Taskfile.yml` / `build/` — Wails v3 build configuration.

Settings (including window geometry) persist to
`<UserConfigDir>/kutyaguru/settings.json`.
