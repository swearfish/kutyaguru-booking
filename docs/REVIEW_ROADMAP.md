# Code Review Roadmap

Tracking checklist for the maintainability/architecture review of the `booking.go`
Wails app. Items are grouped by priority. None are correctness bugs — this is
cleanup to keep the codebase healthy as it grows.

Status legend: `[ ]` todo · `[~]` in progress · `[x]` done

---

## Priority 1 — act on before the codebase grows

- [ ] **Split the `Booking` god-object** (`booking.go:135`)
  Extract the six mixed concerns into focused units:
  - [ ] `document` — `rows` / `rowEnabled` / `columnNames` / `cellErrors` + their mutators
  - [ ] `settingsStore` — load/save + the per-field setters
  - [ ] Excel import/export as free functions taking the document
  - [ ] CSV render as a free function taking the document
  - [ ] `Booking` becomes a thin Wails facade delegating to the above

- [ ] **Guard `b.settings` against data races** (`main.go:55-60`, `booking.go:223`)
  Window events mutate `settings.Window*` while service calls marshal/mutate the
  same struct (incl. maps). Add a `sync.Mutex` around settings access, or confirm
  Wails serializes all callbacks onto one goroutine and document that.

- [ ] **Remove the dead `SaveSettings(s Settings)` full-replace setter** (`booking.go:263`)
  Frontend deliberately never calls it (see `App.tsx:216`); it can still wipe
  server-managed fields if reintroduced. Delete the bound method.

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

- [ ] **Unify `pushRecent` / `removeRecent` idioms** (`:357`, `:373`)
  One allocates fresh, the other filters in place via `RecentFiles[:0]`. Pick the
  clearer allocating style for a 10-element list.

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
