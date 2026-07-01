# Code Review Roadmap

Tracking checklist for the maintainability/architecture review of the `booking.go`
Wails app. Items are grouped by priority. None are correctness bugs — this is
cleanup to keep the codebase healthy as it grows.

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done

---

## Priority 1 — act on before the codebase grows

- [x] **Split the `Booking` god-object** (`booking.go:135`)
  Extract the six mixed concerns into focused units:
  - [x] `document` — `rows` / `rowEnabled` / `columnNames` / `cellErrors` + their mutators (`document.go`)
  - [x] `settingsStore` — load/save persistence boundary (`settings.go`)
  - [x] Excel import/export as free functions on primitives (`excel.go`)
  - [x] CSV render as a `document` method taking template + char mapping (`document.go`)
  - [x] `Booking` becomes a thin Wails facade delegating to the above

- [x] **Guard `b.settings` against data races** (`main.go:55-60`, `booking.go`)
  Confirmed (from the Wails v3 source) bound calls run one-goroutine-per-request
  via `net/http`, concurrent with the native window-event loop → the race is
  real. Added `Booking.mu sync.Mutex` guarding every settings read/mutate/marshal;
  setters use a lock-free `persist()` to do "mutate + flush" as one critical
  section. A dedicated `TestSettingsConcurrentAccess` hammers the write-vs-marshal
  path from 150 goroutines so `go test -race` actually exercises the lock (the
  functional tests are single-goroutine and wouldn't). `doc` stays unguarded
  (bound-call only, UI serialised) — documented on the struct.

- [x] **Remove the dead `SaveSettings(s Settings)` full-replace setter**
  Frontend never called it (only generated bindings referenced it). Deleted the
  bound method and regenerated the Wails bindings (`wails3 generate bindings`,
  now 23 methods) — a 7-line drop in `frontend/bindings/kutyaguru/booking.ts`,
  no other bindings touched.

## Priority 2 — high-value maintainability cleanup

- [ ] **Collapse the triple source of truth for column names**
  Same Hungarian names live in Go consts (`booking.go:40-47`), TS literals
  (`App.tsx:42-43`), and `fields.yaml`. Make YAML/bindings authoritative; stop
  re-typing the literals in the frontend.

- [ ] **De-duplicate column-name → index lookups**
  Build one `map[string]int` when `columnNames` is set; reuse it across
  `getCellByColName` (`:740`), `colIndex` (`:750`), `LoadSheet` (`:407`),
  `ReapplyFields` (`:464`), `UpdateCell` (`:485`), `readExcelFrom` (`:640`).

- [ ] **Drop the unnecessary deep-copies in `buildResult`** (`booking.go:720-738`)
  Result is JSON-marshaled by Wails immediately; the slice copies are dead work
  in the production path.

- [x] **Unify `pushRecent` / `removeRecent` idioms** (`:357`, `:373`)
  Both now allocate a fresh slice (the in-place `RecentFiles[:0]` filter is gone);
  done incidentally while adding the settings mutex.

- [ ] **Document `writeCSV`'s template-format coupling** (`booking.go:872`)
  Add a comment: the leading record number attaches to the first col-def line
  only; later lines start with the template's empty first cell, and
  `ColDefLines[0]` is assumed to be the partner line.

## Priority 3 — frontend

- [ ] **Stop rebuilding all column defs on every cell edit** (`DataTab.tsx:53-163`)
  The `columns` memo depends on `tableData.rows` only for the header checkbox
  tri-state. Split that into its own memo so data-column defs depend only on
  `columns` + error maps.

- [ ] **Fix the mount `useEffect` deps** (`App.tsx:68-80`)
  Add `mantineSetColorScheme` (stable, so harmless) or an explicit
  eslint-disable comment to document intent.

## Priority 4 — minor / nits

- [ ] Log a warning when `init()` falls back to `os.TempDir()` so the
  non-persisting-settings degradation is visible (`booking.go:153-156`).
- [ ] `GetSettings()` ships the whole `Settings` struct though the frontend reads
  a subset (`App.tsx:69-79`) — consider a narrower DTO.
- [ ] Include `rowEnabled` in `emptyTable` (`App.tsx:40`) to drop some defensive
  `rowEnabled?.[i] ?? true` chains.
- [ ] Document the Makefile Windows cross-build path in the README (currently only
  the `wails3` path is documented).
- [ ] Prune sibling clutter from the workspace: `booking.go.v2-backup/`,
  `booking.py/`, `.idea/` (outside this module, but noise).

---

_Generated from the architecture/maintainability review. See git history or the
review notes for the full rationale behind each item._
