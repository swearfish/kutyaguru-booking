package main

import "testing"

// TestColumnConstsExistInSchema guards the coupling between the Go output-column
// consts (which drive validation and per-service pricing) and the column schema
// derived from fields.yaml. If a column is renamed in fields.yaml without updating
// its const — or vice versa — the const would silently stop matching any column
// and its rule would quietly no-op (no error, wrong output). This fails loudly
// instead, making fields.yaml the single source the consts must track.
func TestColumnConstsExistInSchema(t *testing.T) {
	b := newTestBooking(t) // schema built from defaultFieldsYAML, as production init() does
	for _, col := range []string{colService, colPrice, colPartner, colEmail, colPostal, colQuantity} {
		if b.doc.colIndex(col) < 0 {
			t.Errorf("column const %q not found in the fields.yaml schema (rename drift?)", col)
		}
	}
}
