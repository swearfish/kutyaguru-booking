package main

import (
	"testing"
)

// findCellError returns the (single) CellError for a row/column, or false.
func findCellError(errs []CellError, rowIndex int, colName string) (CellError, bool) {
	for _, ce := range errs {
		if ce.RowIndex == rowIndex && ce.ColName == colName {
			return ce, true
		}
	}
	return CellError{}, false
}

func TestValidateContent(t *testing.T) {
	cases := []struct {
		name    string
		col     string
		value   string
		wantBad bool
	}{
		{"partner ok", colPartner, "Kovacs Rita", false},
		{"partner empty", colPartner, "   ", true},
		{"email ok", colEmail, "a@b.hu", false},
		{"email empty", colEmail, "", true},
		{"email no domain", colEmail, "abc", true},
		{"email no tld", colEmail, "a@b", true},
		{"postal ok", colPostal, "1234", false},
		{"postal short", colPostal, "123", true},
		{"postal letters", colPostal, "12a4", true},
		{"quantity ok", colQuantity, "3", false},
		{"quantity zero", colQuantity, "0", true},
		{"quantity neg", colQuantity, "-1", true},
		{"quantity nan", colQuantity, "x", true},
		{"unchecked column", "Település", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ce, bad := validateContent(0, tc.col, tc.value)
			if bad != tc.wantBad {
				t.Fatalf("validateContent(%q,%q) bad=%v, want %v", tc.col, tc.value, bad, tc.wantBad)
			}
			if bad && ce.Severity != severityWarning {
				t.Errorf("severity = %q, want %q", ce.Severity, severityWarning)
			}
		})
	}
}

func TestApplyServicePrices(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	svcIdx := b.doc.colIndex(colService)
	priceIdx := b.doc.colIndex(colPrice)
	if svcIdx < 0 || priceIdx < 0 {
		t.Fatalf("columns not found: svc=%d price=%d", svcIdx, priceIdx)
	}

	// Price only the first row's service; others keep the default.
	targetSvc := b.doc.rows[0][svcIdx]
	defaultPrice := b.doc.rows[0][priceIdx]
	res := b.SetServicePrices(map[string]string{targetSvc: "9 999 Ft"}, "all")

	for ri, row := range res.Rows {
		got := row[priceIdx]
		if row[svcIdx] == targetSvc {
			if got != "9 999 Ft" {
				t.Errorf("row %d priced service: got %q, want %q", ri, got, "9 999 Ft")
			}
		} else if got != defaultPrice {
			t.Errorf("row %d unpriced service: got %q, want default %q", ri, got, defaultPrice)
		}
	}
}

func TestUnpricedServiceWarning(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	// With no prices configured every row's price cell should warn.
	for ri := range b.doc.rows {
		ce, ok := findCellError(b.doc.cellErrors, ri, colPrice)
		if !ok || ce.Severity != severityWarning {
			t.Errorf("row %d: expected unpriced-service warning, got %+v (ok=%v)", ri, ce, ok)
		}
	}

	// After pricing the first row's service, that row no longer warns.
	svcIdx := b.doc.colIndex(colService)
	res := b.SetServicePrices(map[string]string{b.doc.rows[0][svcIdx]: "1 000 Ft"}, "all")
	if ce, ok := findCellError(res.CellErrors, 0, colPrice); ok && ce.Severity == severityWarning {
		t.Errorf("row 0 still warns after pricing: %+v", ce)
	}
}

func TestSeverityClassification(t *testing.T) {
	b := newTestBooking(t)
	b.doc.setColumns([]string{"Megjegyzés"}) // a column not subject to content rules
	b.settings.ServicePrices = map[string]string{}

	// Unmappable char (no mapping) → error (blocks export).
	b.settings.CharMapping = map[string]string{}
	b.doc.rows = [][]string{{"árvíztűrő ▲"}} // ▲ is not in ISO-8859-2
	errs := b.doc.validate(b.settings)
	if ce, ok := findCellError(errs, 0, "Megjegyzés"); !ok || ce.Severity != severityError {
		t.Errorf("unmapped char: got %+v (ok=%v), want severity %q", ce, ok, severityError)
	}

	// Same char, now mapped → mapped (yellow, does not block).
	b.settings.CharMapping = map[string]string{"▲": "^"}
	errs = b.doc.validate(b.settings)
	if ce, ok := findCellError(errs, 0, "Megjegyzés"); !ok || ce.Severity != severityMapped {
		t.Errorf("mapped char: got %+v (ok=%v), want severity %q", ce, ok, severityMapped)
	}
}

// TestBlankMappingUnblocksExport pins Feature 2's core mechanism: a char mapped to
// an EMPTY replacement must flip severity error→mapped (yellow, non-blocking), exactly
// like a non-empty mapping. The "add missing chars as blanks, open Mapping view" flow
// depends on this — validateCell keys on map membership, not on the value being set.
func TestBlankMappingUnblocksExport(t *testing.T) {
	b := newTestBooking(t)
	b.doc.setColumns([]string{"Megjegyzés"}) // a column with no content rules
	b.settings.ServicePrices = map[string]string{}
	b.doc.rows = [][]string{{"árvíztűrő ▲"}} // ▲ is not encodable in ISO-8859-2

	// No mapping → error (blocks export).
	b.settings.CharMapping = map[string]string{}
	if ce, ok := findCellError(b.doc.validate(b.settings), 0, "Megjegyzés"); !ok || ce.Severity != severityError {
		t.Fatalf("unmapped char: got %+v (ok=%v), want severity %q", ce, ok, severityError)
	}

	// Blank mapping (what handleConfirmMapFix adds) → mapped (yellow, does not block).
	b.settings.CharMapping = map[string]string{"▲": ""}
	ce, ok := findCellError(b.doc.validate(b.settings), 0, "Megjegyzés")
	if !ok || ce.Severity != severityMapped {
		t.Fatalf("blank-mapped char: got %+v (ok=%v), want severity %q", ce, ok, severityMapped)
	}
	if ce.MappedTo != "" {
		t.Errorf("MappedTo = %q, want empty (char dropped on export)", ce.MappedTo)
	}
}

// TestApplyFieldToRowsNarrow pins Feature 5's key invariant: applying one field's
// changed default to loaded rows must touch ONLY that field's column, leaving another
// field the user chose not to apply on its previous value. This is why ApplyFieldToRows
// exists instead of reusing ReapplyFields (which rewrites every non-MAPPING column).
func TestApplyFieldToRowsNarrow(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	const applied, untouched = "Mennyiségi egység", "Fizetési mód"
	appliedIdx, untouchedIdx := b.doc.colIndex(applied), b.doc.colIndex(untouched)
	if appliedIdx < 0 || untouchedIdx < 0 {
		t.Fatalf("columns not found: %s=%d %s=%d", applied, appliedIdx, untouched, untouchedIdx)
	}
	// The untouched column's current values (as loaded) — must survive unchanged.
	before := make([]string, len(b.doc.rows))
	for ri, row := range b.doc.rows {
		before[ri] = row[untouchedIdx]
	}

	// Change BOTH defaults, but apply only one (mode "all").
	prevApplied := b.doc.rows[0][appliedIdx]
	if err := b.UpdateFieldValue(applied, "csomag"); err != nil {
		t.Fatal(err)
	}
	if err := b.UpdateFieldValue(untouched, "Átutalás"); err != nil {
		t.Fatal(err)
	}
	res := b.ApplyFieldToRows(applied, "all", prevApplied)

	for ri, row := range res.Rows {
		if row[appliedIdx] != "csomag" {
			t.Errorf("row %d applied column: got %q, want %q", ri, row[appliedIdx], "csomag")
		}
		if row[untouchedIdx] != before[ri] {
			t.Errorf("row %d untouched column changed: got %q, want %q (declined field must not be clobbered)", ri, row[untouchedIdx], before[ri])
		}
	}

	// A MAPPING field name is a no-op (does not rewrite the source column).
	mapCol := "Partner megnevezése:"
	mapIdx := b.doc.colIndex(mapCol)
	snapshot := b.doc.rows[0][mapIdx]
	b.ApplyFieldToRows(mapCol, "all", "")
	if b.doc.rows[0][mapIdx] != snapshot {
		t.Errorf("ApplyFieldToRows on MAPPING field mutated source column: %q → %q", snapshot, b.doc.rows[0][mapIdx])
	}
}

// TestApplyFieldToRowsMatch pins the "match" mode: only rows still carrying the old
// default (prevValue) are rewritten; a manually-edited row is left untouched.
func TestApplyFieldToRowsMatch(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	const fieldName = "Mennyiségi egység" // default "db", applied to every row on load
	idx := b.doc.colIndex(fieldName)
	if idx < 0 || len(b.doc.rows) < 2 {
		t.Fatalf("need column and >=2 rows: idx=%d rows=%d", idx, len(b.doc.rows))
	}
	oldDefault := b.doc.rows[0][idx] // "db"

	// Hand-edit row 1's cell so it no longer matches the old default.
	b.UpdateCell(1, fieldName, "kézi")

	// Change the default and apply in "match" mode.
	if err := b.UpdateFieldValue(fieldName, "csomag"); err != nil {
		t.Fatal(err)
	}
	res := b.ApplyFieldToRows(fieldName, "match", oldDefault)

	for ri, row := range res.Rows {
		if ri == 1 {
			if row[idx] != "kézi" {
				t.Errorf("row 1 (manually edited) got overwritten: %q, want %q", row[idx], "kézi")
			}
		} else if row[idx] != "csomag" {
			t.Errorf("row %d (untouched) not updated: %q, want %q", ri, row[idx], "csomag")
		}
	}
}

// TestApplyFieldToRowsMatchSelect guards the assumption match mode relies on: that
// every field type (not just TEXT) is seeded with its default into all rows on load,
// so a row's "still on the old default" test is meaningful. Select fields flow through
// the same path — if load ever stopped seeding them, match would silently no-op here.
func TestApplyFieldToRowsMatchSelect(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	const fieldName = "Fizetési mód" // Select, default "Készpénz", seeded to every row
	idx := b.doc.colIndex(fieldName)
	if idx < 0 || len(b.doc.rows) < 2 {
		t.Fatalf("need column and >=2 rows: idx=%d rows=%d", idx, len(b.doc.rows))
	}
	oldDefault := b.doc.rows[0][idx]
	if oldDefault == "" {
		t.Fatalf("Select column not seeded on load — match mode would no-op")
	}

	b.UpdateCell(1, fieldName, "Bankkártya") // hand-edit one row off the default

	if err := b.UpdateFieldValue(fieldName, "Átutalás"); err != nil {
		t.Fatal(err)
	}
	res := b.ApplyFieldToRows(fieldName, "match", oldDefault)

	for ri, row := range res.Rows {
		if ri == 1 {
			if row[idx] != "Bankkártya" {
				t.Errorf("row 1 (manually edited) got overwritten: %q, want %q", row[idx], "Bankkártya")
			}
		} else if row[idx] != "Átutalás" {
			t.Errorf("row %d (untouched) not updated: %q, want %q", ri, row[idx], "Átutalás")
		}
	}
}

// TestApplyModeRoundTrip pins the ApplyMode persist→load path, mirroring the
// FieldValues round-trip: SetApplyMode must reach disk and survive a reload.
func TestApplyModeRoundTrip(t *testing.T) {
	b := newTestBooking(t)

	b.SetApplyMode("match")
	if b.settings.ApplyMode != "match" {
		t.Fatalf("ApplyMode not recorded in memory: %q", b.settings.ApplyMode)
	}
	if reloaded := b.store.load(); reloaded.ApplyMode != "match" {
		t.Fatalf("ApplyMode not persisted to disk: %q", reloaded.ApplyMode)
	}

	// An empty value from an older settings file backfills to the "ask" default.
	b.SetApplyMode("")
	if got := b.store.load().ApplyMode; got != "ask" {
		t.Errorf("empty ApplyMode did not backfill to default: %q", got)
	}
}

// TestSetServicePricesMatch pins the price "match" mode: changing a service's price
// rewrites only rows still on that service's previous price, leaving hand-edited rows.
// Built on a synthetic document so the same service appears on multiple rows (the real
// fixture has no repeated service) and the untouched/edited split is exercisable.
func TestSetServicePricesMatch(t *testing.T) {
	b := newTestBooking(t)
	b.doc.setColumns([]string{colService, colPrice})
	b.settings.ServicePrices = map[string]string{}
	// Two rows share service "Alap", one row is a different service.
	b.doc.rows = [][]string{
		{"Alap", "100"},
		{"Alap", "100"},
		{"Prémium", "200"},
	}
	svcIdx, priceIdx := b.doc.colIndex(colService), b.doc.colIndex(colPrice)

	// Configure "Alap" at 100, then hand-edit row 1 off it.
	b.SetServicePrices(map[string]string{"Alap": "100"}, "all")
	b.UpdateCell(1, colPrice, "kézi ár")

	// Change "Alap" to 150 in "match" mode.
	res := b.SetServicePrices(map[string]string{"Alap": "150"}, "match")

	want := map[int]string{0: "150", 1: "kézi ár", 2: "200"}
	for ri := range res.Rows {
		if got := res.Rows[ri][priceIdx]; got != want[ri] {
			t.Errorf("row %d (%s) price = %q, want %q", ri, res.Rows[ri][svcIdx], got, want[ri])
		}
	}
}

// TestSetServicePricesMatchFromFlatDefault pins the match sub-path where a service was
// previously UNPRICED: its rows' effective old price is the flat "Nettó egységár" default
// (read from b.fields), so match rewrites rows still showing that default and leaves
// hand-edited rows. This is the only match path that exercises the flatDefault lookup.
func TestSetServicePricesMatchFromFlatDefault(t *testing.T) {
	b := newTestBooking(t)
	b.doc.setColumns([]string{colService, colPrice})
	b.settings.ServicePrices = map[string]string{}
	for i := range b.fields {
		if b.fields[i].Name == colPrice {
			b.fields[i].Value = "100" // flat default the unpriced rows currently show
		}
	}
	b.doc.rows = [][]string{
		{"Alap", "100"},      // still on the flat default → match rewrites
		{"Alap", "kézi ár"},  // hand-edited → preserved
	}
	priceIdx := b.doc.colIndex(colPrice)

	// "Alap" was never priced; price it in match mode.
	res := b.SetServicePrices(map[string]string{"Alap": "150"}, "match")

	if got := res.Rows[0][priceIdx]; got != "150" {
		t.Errorf("row 0 (on flat default) not updated: %q, want %q", got, "150")
	}
	if got := res.Rows[1][priceIdx]; got != "kézi ár" {
		t.Errorf("row 1 (hand-edited) overwritten: %q, want %q", got, "kézi ár")
	}
}

// TestSetServicePricesNever pins "never": the lookup is persisted (so unpriced-service
// warnings clear) but loaded price cells are left untouched.
func TestSetServicePricesNever(t *testing.T) {
	b := newTestBooking(t)
	b.doc.setColumns([]string{colService, colPrice})
	b.settings.ServicePrices = map[string]string{}
	b.doc.rows = [][]string{{"Alap", "100"}}

	res := b.SetServicePrices(map[string]string{"Alap": "999"}, "never")
	if got := res.Rows[0][b.doc.colIndex(colPrice)]; got != "100" {
		t.Errorf("never mode rewrote the price cell: got %q, want %q", got, "100")
	}
	if b.settings.ServicePrices["Alap"] != "999" {
		t.Errorf("never mode did not persist the lookup: %v", b.settings.ServicePrices)
	}
	// The unpriced-service warning must be gone now that "Alap" has a price.
	if _, ok := findCellError(res.CellErrors, 0, colPrice); ok {
		t.Errorf("unpriced warning still present after pricing in never mode")
	}
}

// TestApplyFieldToRowsPreservesPriceChoice pins the durability-trap fix: applying an
// unrelated field default must NOT re-assert configured service prices onto rows a
// prior price choice left off the configured value.
func TestApplyFieldToRowsPreservesPriceChoice(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	svcIdx := b.doc.colIndex(colService)
	priceIdx := b.doc.colIndex(colPrice)
	svc := b.doc.rows[0][svcIdx]

	// Configure a price, then hand-edit row 0 off it.
	b.SetServicePrices(map[string]string{svc: "5 000 Ft"}, "all")
	b.UpdateCell(0, colPrice, "egyedi") // manual override on row 0

	// Apply an UNRELATED field default across all rows.
	const other = "Mennyiségi egység"
	if b.doc.colIndex(other) < 0 {
		t.Fatalf("column %q not found", other)
	}
	prev := b.doc.rows[0][b.doc.colIndex(other)]
	if err := b.UpdateFieldValue(other, "csomag"); err != nil {
		t.Fatal(err)
	}
	res := b.ApplyFieldToRows(other, "all", prev)

	if got := res.Rows[0][priceIdx]; got != "egyedi" {
		t.Errorf("row 0 price clobbered by unrelated field apply: got %q, want %q", got, "egyedi")
	}
}

func TestFieldValuesRoundTrip(t *testing.T) {
	b := newTestBooking(t)

	// Editable TEXT field is persisted and saved.
	if err := b.UpdateFieldValue("Fizetési mód", "Átutalás"); err != nil {
		t.Fatalf("UpdateFieldValue: %v", err)
	}
	if b.settings.FieldValues["Fizetési mód"] != "Átutalás" {
		t.Fatalf("FieldValues not recorded: %v", b.settings.FieldValues)
	}

	// Reload settings from disk and re-apply onto freshly parsed fields.
	reloaded := b.store.load()
	if reloaded.FieldValues["Fizetési mód"] != "Átutalás" {
		t.Fatalf("FieldValues not persisted to disk: %v", reloaded.FieldValues)
	}

	b2 := newBooking()
	b2.settings = reloaded
	fields, err := parseFieldsYAML(defaultFieldsYAML)
	if err != nil {
		t.Fatal(err)
	}
	b2.fields = fields
	b2.restoreFieldValues()

	for _, f := range b2.fields {
		if f.Name == "Fizetési mód" && f.Value != "Átutalás" {
			t.Errorf("restoreFieldValues: got %q, want %q", f.Value, "Átutalás")
		}
		// DATE fields must stay computed-from-today, never restored from settings.
		if f.Type == FieldTypeDate {
			if _, ok := b2.settings.FieldValues[f.Name]; ok {
				t.Errorf("DATE field %q was persisted to FieldValues", f.Name)
			}
		}
	}
}
