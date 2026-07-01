package main

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// excel.go isolates every excelize dependency: opening workbooks, reading sheets
// into the output schema, and the xlsx round-trip that carries the per-row on/off
// flag column. These are free functions on primitives so the document model stays
// free of any file-format knowledge (the caller wires the results into its state).

// sheetNames opens an xlsx workbook and returns its sheet names. Used by both the
// file-open dialog and the recent-file reopen paths, which need nothing more.
func sheetNames(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()
	return f.GetSheetList(), nil
}

// readBookedSheet reads the named sheet from a Booked4us xlsx and projects each
// data row onto the output schema described by fields: MAPPING fields are pulled
// from the matching source column (by header name), all other field types take
// their constant/default value. Returns an empty (non-nil-safe) slice for a
// header-only or empty sheet.
func readBookedSheet(path, sheetName string, fields []Field) ([][]string, error) {
	f, err := excelize.OpenFile(path, excelize.Options{RawCellValue: false})
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	allRows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("munkalap olvasási hiba: %w", err)
	}
	if len(allRows) == 0 {
		return nil, nil
	}

	headerRow := allRows[0]
	headerIndex := make(map[string]int, len(headerRow))
	for i, h := range headerRow {
		headerIndex[h] = i
	}

	rows := make([][]string, 0, len(allRows)-1)
	for _, dataRow := range allRows[1:] {
		row := make([]string, len(fields))
		for i, field := range fields {
			switch field.Type {
			case FieldTypeMapping:
				if idx, ok := headerIndex[field.Mapping]; ok && idx < len(dataRow) {
					row[i] = dataRow[idx]
				}
			default:
				row[i] = field.Value
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// writeExcelFile writes the data columns plus a trailing on/off flag column
// (colEnabledHeader) to an xlsx file. Every row is written — Excel is a
// working-data save — and the flag column records each row's state so a
// re-import restores the toggles. A nil or short rowEnabled writes as all-on,
// matching document.rowIsEnabled's default-true guard.
func writeExcelFile(path string, columnNames []string, rows [][]string, rowEnabled []bool) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	for ci, name := range columnNames {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		f.SetCellValue(sheet, cell, name)
	}
	flagCol := len(columnNames) + 1
	if cell, err := excelize.CoordinatesToCellName(flagCol, 1); err == nil {
		f.SetCellValue(sheet, cell, colEnabledHeader)
	}
	for ri, row := range rows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellValue(sheet, cell, val)
		}
		flag := "0"
		if ri >= len(rowEnabled) || rowEnabled[ri] {
			flag = "1"
		}
		if cell, err := excelize.CoordinatesToCellName(flagCol, ri+2); err == nil {
			f.SetCellValue(sheet, cell, flag)
		}
	}
	return f.SaveAs(path)
}

// readExcelFile loads a previously exported xlsx back onto the output schema.
// Columns are matched by name against columnNames, so the flag column
// (colEnabledHeader) and any extra columns never leak into the data. A row is
// enabled unless the flag column explicitly holds "0"; files without that column
// (plain Booked4us exports) load as all-on.
func readExcelFile(path string, columnNames []string) (rows [][]string, rowEnabled []bool, err error) {
	f, err := excelize.OpenFile(path, excelize.Options{RawCellValue: false})
	if err != nil {
		return nil, nil, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, nil, fmt.Errorf("üres munkafüzet")
	}
	allRows, err := f.GetRows(sheets[0])
	if err != nil || len(allRows) == 0 {
		return nil, nil, fmt.Errorf("munkalap olvasási hiba: %w", err)
	}

	importCols := allRows[0]
	importIndex := make(map[string]int, len(importCols))
	for i, c := range importCols {
		importIndex[c] = i
	}
	enabledIdx, hasEnabled := importIndex[colEnabledHeader]

	rows = make([][]string, 0, len(allRows)-1)
	rowEnabled = make([]bool, 0, len(allRows)-1)
	for _, dataRow := range allRows[1:] {
		row := make([]string, len(columnNames))
		for ci, name := range columnNames {
			if idx, ok := importIndex[name]; ok && idx < len(dataRow) {
				row[ci] = dataRow[idx]
			}
		}
		rows = append(rows, row)
		enabled := true
		if hasEnabled && enabledIdx < len(dataRow) {
			enabled = strings.TrimSpace(dataRow[enabledIdx]) != "0"
		}
		rowEnabled = append(rowEnabled, enabled)
	}
	return rows, rowEnabled, nil
}
