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
	res := b.SetServicePrices(map[string]string{targetSvc: "9 999 Ft"})

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
	res := b.SetServicePrices(map[string]string{b.doc.rows[0][svcIdx]: "1 000 Ft"})
	if ce, ok := findCellError(res.CellErrors, 0, colPrice); ok && ce.Severity == severityWarning {
		t.Errorf("row 0 still warns after pricing: %+v", ce)
	}
}

func TestSeverityClassification(t *testing.T) {
	b := newTestBooking(t)
	b.doc.columnNames = []string{"Megjegyzés"} // a column not subject to content rules
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
