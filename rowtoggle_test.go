package main

import (
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"testing"
)

// recordNumRe matches the leading sequence number of each CSV record's first
// line (e.g. "1;Kovacs Rita;..."). Template header/coldef lines begin with an
// empty first cell (";…"), so they never match.
var recordNumRe = regexp.MustCompile(`(?m)^(\d+);`)

func recordNumbers(t *testing.T, b *Booking) []int {
	t.Helper()
	csv, err := b.PreviewCSV()
	if err != nil {
		t.Fatalf("PreviewCSV: %v", err)
	}
	var nums []int
	for _, m := range recordNumRe.FindAllStringSubmatch(csv, -1) {
		n, _ := strconv.Atoi(m[1])
		nums = append(nums, n)
	}
	return nums
}

// TestCSVRenumberSkipsDisabled is the core behavior: a disabled row is excluded
// from the CSV and the remaining rows are renumbered contiguously (1,2 — not 1,3).
func TestCSVRenumberSkipsDisabled(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	// Baseline: all three rows on → numbered 1,2,3.
	if got, want := recordNumbers(t, b), []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("baseline numbering: got %v, want %v", got, want)
	}

	// Disable the middle row (Toth Laszlo).
	b.SetRowEnabled(1, false)

	if got, want := recordNumbers(t, b), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("renumbering after disabling middle row: got %v, want %v (off-by-one?)", got, want)
	}

	csv, _ := b.PreviewCSV()
	if regexp.MustCompile(`Toth Laszlo`).MatchString(csv) {
		t.Error("disabled row data leaked into CSV")
	}
	for _, want := range []string{"Kovacs Rita", "Nagy Anna"} {
		if !regexp.MustCompile(want).MatchString(csv) {
			t.Errorf("enabled row %q missing from CSV", want)
		}
	}
}

// TestExcelRoundTripPreservesToggles verifies the _Aktív flag column survives an
// export → re-import, and that it never leaks into the data columns/rows.
func TestExcelRoundTripPreservesToggles(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}
	b.SetRowEnabled(0, false)
	b.SetRowEnabled(2, false)

	path := filepath.Join(t.TempDir(), "roundtrip.xlsx")
	if err := b.writeExcelTo(path); err != nil {
		t.Fatalf("writeExcelTo: %v", err)
	}

	b2 := newTestBooking(t)
	if err := b2.readExcelFrom(path); err != nil {
		t.Fatalf("readExcelFrom: %v", err)
	}

	// Toggles restored on the same rows.
	if got, want := b2.doc.rowEnabled, []bool{false, true, false}; !reflect.DeepEqual(got, want) {
		t.Errorf("rowEnabled after round-trip: got %v, want %v", got, want)
	}
	// _Aktív must not leak into the schema or the row data.
	if len(b2.doc.columnNames) != len(b.doc.columnNames) {
		t.Errorf("column count changed: got %d, want %d (flag column leaked?)", len(b2.doc.columnNames), len(b.doc.columnNames))
	}
	if len(b2.doc.rows) != len(b.doc.rows) {
		t.Fatalf("row count changed: got %d, want %d", len(b2.doc.rows), len(b.doc.rows))
	}
	for ri := range b2.doc.rows {
		if len(b2.doc.rows[ri]) != len(b.doc.columnNames) {
			t.Errorf("row %d width %d, want %d (flag column leaked into data)", ri, len(b2.doc.rows[ri]), len(b.doc.columnNames))
		}
		if !reflect.DeepEqual(b2.doc.rows[ri], b.doc.rows[ri]) {
			t.Errorf("row %d data differs after round-trip:\n got:  %q\n want: %q", ri, b2.doc.rows[ri], b.doc.rows[ri])
		}
	}
}

// TestDisabledRowDoesNotBlockExport verifies an un-encodable cell in a DISABLED
// row no longer blocks CSV export (and that an enabled one still does).
func TestDisabledRowDoesNotBlockExport(t *testing.T) {
	b := newTestBooking(t)
	b.doc.columnNames = []string{"Megjegyzés"}
	b.settings.Encoding = "ISO-8859-2"
	b.settings.CharMapping = map[string]string{}
	b.doc.rows = [][]string{{"árvíztűrő ▲"}} // ▲ is not in ISO-8859-2
	b.doc.rowEnabled = newEnabledSlice(len(b.doc.rows))
	b.doc.cellErrors = b.doc.validate(b.settings)

	// Enabled: the error blocks export.
	if err := b.doc.blockingExportError(); err == nil {
		t.Fatal("expected export to be blocked while the error row is enabled")
	}

	// Disabled: the same error no longer blocks export.
	b.SetRowEnabled(0, false)
	if err := b.doc.blockingExportError(); err != nil {
		t.Errorf("disabled error row should not block export, got: %v", err)
	}

	// And the disabled row is absent from the rendered CSV.
	if nums := recordNumbers(t, b); len(nums) != 0 {
		t.Errorf("disabled row rendered into CSV: record numbers %v", nums)
	}
}
