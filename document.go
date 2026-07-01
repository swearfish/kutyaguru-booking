package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// document is the in-memory table model: the output column schema plus the
// current rows, their on/off export flags, and the cell-level validation
// results. It owns all operations that read or mutate this state; settings-
// dependent behaviour (validation, pricing, CSV rendering) takes the relevant
// settings as explicit parameters so the model stays decoupled from where those
// preferences are stored.
type document struct {
	columnNames []string
	colIdx      map[string]int // name → index into columnNames; rebuilt by setColumns
	rows        [][]string
	rowEnabled  []bool // parallel to rows; true = included in CSV export. In-memory only.
	cellErrors  []CellError
}

// setColumns replaces the output column schema and rebuilds the name→index
// lookup. It is the only writer of columnNames, so colIdx can never go stale.
// Duplicate names keep their first index, matching the old linear-scan lookups.
func (d *document) setColumns(names []string) {
	d.columnNames = names
	d.colIdx = make(map[string]int, len(names))
	for i, name := range names {
		if _, exists := d.colIdx[name]; !exists {
			d.colIdx[name] = i
		}
	}
}

// colIndex returns the column index for the given output column name, or -1.
func (d *document) colIndex(colName string) int {
	if i, ok := d.colIdx[colName]; ok {
		return i
	}
	return -1
}

func (d *document) getCellByColName(row []string, colName string) string {
	if i, ok := d.colIdx[colName]; ok && i < len(row) {
		return row[i]
	}
	return ""
}

// newEnabledSlice returns a length-n slice with every row enabled. (make([]bool,n)
// is all-false, so the flags must be set explicitly.)
func newEnabledSlice(n int) []bool {
	s := make([]bool, n)
	for i := range s {
		s[i] = true
	}
	return s
}

// rowIsEnabled reports whether row i is included in export. A nil or too-short
// rowEnabled (e.g. tests that assign rows directly) counts as enabled, so no
// path can panic or silently drop every row.
func (d *document) rowIsEnabled(i int) bool {
	return i >= len(d.rowEnabled) || d.rowEnabled[i]
}

// ensureRowEnabled (re)sizes rowEnabled to match len(rows), preserving existing
// flags and defaulting any new entries to enabled.
func (d *document) ensureRowEnabled() {
	if len(d.rowEnabled) == len(d.rows) {
		return
	}
	next := newEnabledSlice(len(d.rows))
	copy(next, d.rowEnabled)
	d.rowEnabled = next
}

// setRowEnabled toggles whether a single row is included in CSV export.
func (d *document) setRowEnabled(rowIndex int, enabled bool) {
	if rowIndex >= 0 && rowIndex < len(d.rows) {
		d.ensureRowEnabled()
		d.rowEnabled[rowIndex] = enabled
	}
}

// setAllRowsEnabled toggles every row on or off (select-all / select-none).
func (d *document) setAllRowsEnabled(enabled bool) {
	d.rowEnabled = make([]bool, len(d.rows))
	for i := range d.rowEnabled {
		d.rowEnabled[i] = enabled
	}
}

// updateCell mutates one cell by output column name (no-op if out of range).
func (d *document) updateCell(rowIndex int, colName, value string) {
	colIdx := d.colIndex(colName)
	if colIdx >= 0 && rowIndex >= 0 && rowIndex < len(d.rows) {
		d.rows[rowIndex][colIdx] = value
	}
}

// buildResult returns a deep, non-nil copy of the table for the frontend.
func (d *document) buildResult() TableDataResult {
	cols := make([]string, len(d.columnNames))
	copy(cols, d.columnNames)
	rows := make([][]string, len(d.rows))
	for i, r := range d.rows {
		rows[i] = make([]string, len(r))
		copy(rows[i], r)
	}
	errs := make([]CellError, len(d.cellErrors))
	copy(errs, d.cellErrors)
	// Synthesize a fresh, full-length, non-nil slice so the frontend can index it
	// directly (len == len(rows)) and it marshals to [] not null. Missing/short
	// rowEnabled defaults to enabled.
	enabled := make([]bool, len(d.rows))
	for i := range enabled {
		enabled[i] = i >= len(d.rowEnabled) || d.rowEnabled[i]
	}
	return TableDataResult{Columns: cols, Rows: rows, RowEnabled: enabled, CellErrors: errs}
}

// applyServicePrices overwrites each row's net-unit-price cell with the price
// configured for that row's service, when one exists. Rows whose service has no
// configured price keep whatever value is already there (the flat default).
func (d *document) applyServicePrices(prices map[string]string) {
	if len(prices) == 0 {
		return
	}
	svcIdx := d.colIndex(colService)
	priceIdx := d.colIndex(colPrice)
	if svcIdx < 0 || priceIdx < 0 {
		return
	}
	for ri := range d.rows {
		if svcIdx >= len(d.rows[ri]) || priceIdx >= len(d.rows[ri]) {
			continue
		}
		if price, ok := prices[d.rows[ri][svcIdx]]; ok {
			d.rows[ri][priceIdx] = price
		}
	}
}

// severityRank orders the three severities so the most important issue wins
// when several apply to the same cell (error > warning > mapped).
func severityRank(s string) int {
	switch s {
	case severityError:
		return 3
	case severityWarning:
		return 2
	case severityMapped:
		return 1
	default:
		return 0
	}
}

// validate produces one CellError per problematic cell. Encoding checks run only
// outside UTF-8 mode; content-quality and unpriced-service warnings run always
// (content validity is encoding-independent).
func (d *document) validate(s Settings) []CellError {
	checkEncoding := !strings.EqualFold(s.Encoding, "UTF-8")
	svcIdx := d.colIndex(colService)
	priceIdx := d.colIndex(colPrice)

	// Keep the highest-severity issue per cell, keyed by "row:col".
	best := make(map[string]CellError)
	consider := func(ce CellError, ok bool) {
		if !ok {
			return
		}
		key := fmt.Sprintf("%d:%s", ce.RowIndex, ce.ColName)
		if cur, exists := best[key]; !exists || severityRank(ce.Severity) > severityRank(cur.Severity) {
			best[key] = ce
		}
	}

	for ri, row := range d.rows {
		for ci, val := range row {
			colName := d.columnNames[ci]
			if checkEncoding {
				consider(validateCell(ri, colName, val, s.CharMapping))
			}
			consider(validateContent(ri, colName, val))
			// Unpriced-service warning: the net-unit-price cell of a row whose
			// service has no configured price.
			if ci == priceIdx && svcIdx >= 0 && svcIdx < len(row) {
				svc := row[svcIdx]
				if _, priced := s.ServicePrices[svc]; !priced && strings.TrimSpace(svc) != "" {
					consider(CellError{RowIndex: ri, ColName: colName, Value: val,
						Severity: severityWarning,
						Message:  "Nincs ár ehhez a szolgáltatáshoz"}, true)
				}
			}
		}
	}

	errs := make([]CellError, 0, len(best))
	for _, ce := range best {
		errs = append(errs, ce)
	}
	return errs
}

// applyCharMapping substitutes characters in value using the char mapping.
func applyCharMapping(value string, mapping map[string]string) string {
	if len(mapping) == 0 {
		return value
	}
	var sb strings.Builder
	for _, r := range value {
		if repl, ok := mapping[string(r)]; ok {
			sb.WriteString(repl)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// blockingExportError returns a non-nil error if any ENABLED row has an
// un-encodable cell, which must block CSV export. Errors in disabled rows are
// ignored because those rows aren't written. Returns nil when export may proceed.
func (d *document) blockingExportError() error {
	for _, ce := range d.cellErrors {
		if ce.Severity == severityError && d.rowIsEnabled(ce.RowIndex) {
			return fmt.Errorf("nem menthető: a %d. sor %q oszlopában nem kódolható karakter: %q (pozíció: %d)",
				ce.RowIndex+1, ce.ColName, ce.InvalidChar, ce.CharPos)
		}
	}
	return nil
}

// writeCSV renders the Számlázz.hu CSV from the template and enabled rows,
// applying the char mapping to every cell. The leading record number prefixes
// only the first col-def line of each record; subsequent col-def lines start
// with the template's empty first cell.
func (d *document) writeCSV(w io.Writer, tmpl templateData, charMapping map[string]string) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for _, line := range tmpl.HeaderLines {
		fmt.Fprintln(bw, line)
	}
	for _, colDef := range tmpl.ColDefLines {
		fmt.Fprintln(bw, strings.Join(colDef, ";"))
	}
	num := 0
	for i, row := range d.rows {
		if !d.rowIsEnabled(i) {
			continue
		}
		num++
		fmt.Fprintf(bw, "%d", num)
		for _, colDef := range tmpl.ColDefLines {
			for _, colName := range colDef {
				cell := applyCharMapping(d.getCellByColName(row, colName), charMapping)
				fmt.Fprintf(bw, "%s;", cell)
			}
			fmt.Fprintln(bw)
		}
	}
	return nil
}
