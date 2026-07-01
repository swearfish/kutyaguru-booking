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

- [x] **De-duplicate column-name → index lookups**
  `document` now caches a `colIdx map[string]int`, rebuilt by the single
  `setColumns` writer (so it can't go stale). `colIndex`/`getCellByColName` are
  O(1) map lookups, and `ReapplyFields` dropped its own local index map. The
  `headerIndex`/`importIndex` maps in `excel.go` stay — they index a *foreign*
  source/import schema, not our output columns.

- [x] **Drop the unnecessary deep-copies in `buildResult`** — _reconsidered, kept._
  On inspection the roadmap premise doesn't hold: the copies aren't dead work.
  They (a) normalize nil→`[]` so the empty-table JSON marshals to `[]` not `null`
  (the frontend indexes without guards), and (b) decouple the returned DTO from
  later in-place mutation (`updateCell` writes `d.rows[r][c]` directly). Removing
  them would mean aliasing internal mutable state out through the DTO to save a
  tens-of-rows copy on a desktop tool — a latent footgun for zero benefit.
  Recorded the rationale in `buildResult`'s doc comment so it isn't "optimized"
  away later.

- [x] **Unify `pushRecent` / `removeRecent` idioms** (`:357`, `:373`)
  Both now allocate a fresh slice (the in-place `RecentFiles[:0]` filter is gone);
  done incidentally while adding the settings mutex.

- [x] **Document `writeCSV`'s template-format coupling** (`document.go`)
  Expanded the `writeCSV` doc comment: the record number is written once before
  the first col-def line and fills that line's empty leading cell, so
  `ColDefLines[0]` is assumed to be the partner line; later col-def lines carry
  no number and begin with the template's own empty first cell.

## Priority 3 — frontend

- [x] **Split the conflated `columns` memo** (`DataTab.tsx`)
  Split the single `columns` memo into `enabledColumn` (row-toggle checkbox,
  driven by row on/off state) and `dataColumns` (schema + validation maps),
  combined by a cheap third memo. Each now has honest, minimal deps; the split
  also fixed a latent missing dep (`onAddToMapping`, previously covered only
  transitively via `charMapping`). Note: this is a clarity/correctness refactor,
  **not** a rebuild-count win — the backend returns freshly-allocated arrays on
  every mutation, so `tableData.columns`/`rowEnabled`/`cellErrors` change
  reference each edit and both memos still recompute regardless. Truly cutting
  rebuilds would need ref-indirection for the error maps + a stable schema key,
  which isn't worth it for a tens-of-rows grid where perf is a non-concern.

- [x] **Fix the mount `useEffect` deps** (`App.tsx`)
  Added `mantineSetColorScheme` to the dep array (a stable ref from
  `useMantineColorScheme`, so the effect still runs exactly once) and a comment
  documenting the mount-once intent. Preferred over an eslint-disable: the rule
  can't prove Mantine's setter is stable, so listing it is the honest fix.

## Priority 4 — minor / nits

- [x] Log a warning when the settings store falls back to `os.TempDir()` so the
  non-persisting-settings degradation is visible (`settings.go`,
  `newSettingsStore`). Uses the standard `log` package, as `main.go` does.
- [x] `GetSettings()` ships the whole `Settings` struct though the frontend reads
  a subset (`App.tsx`). Added a `UISettings` DTO (colorScheme, encoding,
  charMapping, servicePrices, recentFiles) and returns it from `GetSettings`;
  window geometry and persisted field values are now excluded from the bound API.
  Field names match, so `App.tsx` is unchanged; regenerated bindings dropped the
  now-unreferenced `Settings` model in favour of `UISettings`.
- [x] Include `rowEnabled` in `emptyTable` (`App.tsx`) to drop some defensive
  `rowEnabled?.[i] ?? true` chains. The generated `TableDataResult` already types
  `rowEnabled` as non-optional `boolean[]` and its constructor defaults it to `[]`,
  so `emptyTable` already had it — made it explicit for self-documentation. Dropped
  the now-redundant **null**-guards (`?.`, `?? []`) at the four use sites; kept the
  meaningful **index**-level defaults (`?? true` / `=== false`) which guard
  out-of-bounds access, a separate concern.
- [x] Document the Makefile Windows cross-build path in the README. Added a
  "Cross-building the Windows `.exe` from macOS" subsection covering
  `make windows-setup` / `windows` / `windows ARCH=arm64` and the
  Windows-host-only `windows-package` (NSIS) caveat.
- [ ] Prune sibling clutter from the workspace: `booking.go.v2-backup/`,
  `booking.py/`, `.idea/` (outside this module, but noise).

---

_Generated from the architecture/maintainability review. See git history or the
review notes for the full rationale behind each item._
