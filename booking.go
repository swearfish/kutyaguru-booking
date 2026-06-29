package main

import (
	_ "embed"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
	"gopkg.in/yaml.v3"
)

//go:embed assets/fields.yaml
var defaultFieldsYAML []byte

//go:embed assets/basic_sablon.csv
var templateCSVBytes []byte

// FieldType distinguishes how a column's value is sourced.
type FieldType string

const (
	FieldTypeMapping FieldType = "MAPPING"
	FieldTypeConst   FieldType = "CONST"
	FieldTypeText    FieldType = "TEXT"
	FieldTypeDate    FieldType = "DATE"
)

// Field is serialised to JSON and sent to the frontend for the Mezők tab.
type Field struct {
	Name    string    `json:"name"`
	Type    FieldType `json:"type"`
	Mapping string    `json:"mapping,omitempty"` // source Excel column header (MAPPING only)
	Value   string    `json:"value"`
	Options []string  `json:"options,omitempty"` // dropdown choices (TEXT only)
}

// CellError describes one cell whose value cannot be encoded in ISO-8859-2.
type CellError struct {
	RowIndex    int    `json:"rowIndex"`
	ColName     string `json:"colName"`
	Value       string `json:"value"`
	InvalidChar string `json:"invalidChar"` // the first unencodable rune as a string
	CharPos     int    `json:"charPos"`     // byte offset of that rune within Value
}

// TableDataResult is returned to the frontend whenever the table changes.
type TableDataResult struct {
	Columns    []string    `json:"columns"`
	Rows       [][]string  `json:"rows"`
	CellErrors []CellError `json:"cellErrors"`
}

// templateData holds the parsed Számlázz.hu CSV template.
type templateData struct {
	HeaderLines []string   // lines that started with ";;" — written verbatim to output
	ColDefLines [][]string // each element is one ";"-prefixed line split on ";"
}

// editableFieldYAML is used only while parsing fields.yaml.
type editableFieldYAML struct {
	Type    string   `yaml:"type"`
	Value   string   `yaml:"value"`
	Options []string `yaml:"options"`
	Today   bool     `yaml:"today"`
	Plus    int      `yaml:"plus"`
}

// Booking is the struct bound to Wails. Every exported method becomes callable
// from JavaScript as window.go.main.Booking.<MethodName>().
type Booking struct {
	ctx         context.Context
	fields      []Field      // ordered field definitions
	columnNames []string     // parallel to fields: [f.Name for f in fields]
	rows        [][]string   // one []string per booking row, aligned to columnNames
	tmpl        templateData // parsed basic_sablon.csv
	excelPath   string       // path of the currently open Booked4us file
	cellErrors  []CellError  // current encoding validation errors
}

func newBooking() *Booking { return &Booking{} }

// init is called once from app.startup. Loads embedded assets.
func (b *Booking) init() error {
	fields, err := parseFieldsYAML(defaultFieldsYAML)
	if err != nil {
		return fmt.Errorf("fields.yaml: %w", err)
	}
	b.fields = fields
	b.columnNames = make([]string, len(fields))
	for i, f := range fields {
		b.columnNames[i] = f.Name
	}

	tmpl, err := loadTemplate(templateCSVBytes)
	if err != nil {
		return fmt.Errorf("basic_sablon.csv: %w", err)
	}
	b.tmpl = tmpl
	return nil
}

// ─── Wails-exposed methods ────────────────────────────────────────────────────

// OpenBookedFile shows a file-open dialog for xlsx files and returns the sheet names.
func (b *Booking) OpenBookedFile() ([]string, error) {
	path, err := runtime.OpenFileDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "Booked4us Excel megnyitása",
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel fájlok (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // user cancelled
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	b.excelPath = path
	return f.GetSheetList(), nil
}

// LoadSheet reads the named sheet from the already-opened Excel file.
func (b *Booking) LoadSheet(sheetName string) (TableDataResult, error) {
	if b.excelPath == "" {
		return TableDataResult{}, fmt.Errorf("nincs megnyitott Excel fájl")
	}
	f, err := excelize.OpenFile(b.excelPath, excelize.Options{RawCellValue: false})
	if err != nil {
		return TableDataResult{}, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	allRows, err := f.GetRows(sheetName)
	if err != nil {
		return TableDataResult{}, fmt.Errorf("munkalap olvasási hiba: %w", err)
	}
	if len(allRows) == 0 {
		b.rows = nil
		b.cellErrors = nil
		return b.buildResult(), nil
	}

	// Build header index from the first row.
	headerRow := allRows[0]
	headerIndex := make(map[string]int, len(headerRow))
	for i, h := range headerRow {
		headerIndex[h] = i
	}

	b.rows = make([][]string, 0, len(allRows)-1)
	for _, dataRow := range allRows[1:] {
		row := make([]string, len(b.fields))
		for i, field := range b.fields {
			switch field.Type {
			case FieldTypeMapping:
				if idx, ok := headerIndex[field.Mapping]; ok && idx < len(dataRow) {
					row[i] = dataRow[idx]
				}
			default:
				row[i] = field.Value
			}
		}
		b.rows = append(b.rows, row)
	}

	b.cellErrors = b.validateAllCells()
	return b.buildResult(), nil
}

// GetFields returns the current field definitions for the Mezők tab.
func (b *Booking) GetFields() []Field {
	return b.fields
}

// UpdateFieldValue mutates the in-memory value of an editable field.
// The change takes effect immediately via ReapplyFields on the frontend.
func (b *Booking) UpdateFieldValue(fieldName, value string) error {
	for i := range b.fields {
		if b.fields[i].Name == fieldName {
			if b.fields[i].Type == FieldTypeMapping || b.fields[i].Type == FieldTypeConst {
				return fmt.Errorf("%q mező nem szerkeszthető", fieldName)
			}
			b.fields[i].Value = value
			return nil
		}
	}
	return fmt.Errorf("ismeretlen mező: %q", fieldName)
}

// ReapplyFields re-applies current editable/const/date field values to all
// non-MAPPING columns in every loaded row. MAPPING columns are preserved.
func (b *Booking) ReapplyFields() TableDataResult {
	colIndex := make(map[string]int, len(b.columnNames))
	for i, name := range b.columnNames {
		colIndex[name] = i
	}
	for ri := range b.rows {
		for _, field := range b.fields {
			if field.Type == FieldTypeMapping {
				continue
			}
			if idx, ok := colIndex[field.Name]; ok {
				b.rows[ri][idx] = field.Value
			}
		}
	}
	b.cellErrors = b.validateAllCells()
	return b.buildResult()
}

// UpdateCell mutates one cell and re-validates it in place.
func (b *Booking) UpdateCell(rowIndex int, colName, value string) TableDataResult {
	colIdx := -1
	for i, name := range b.columnNames {
		if name == colName {
			colIdx = i
			break
		}
	}
	if colIdx >= 0 && rowIndex >= 0 && rowIndex < len(b.rows) {
		b.rows[rowIndex][colIdx] = value
	}
	// Re-validate just this cell: remove old error, add new if any.
	filtered := b.cellErrors[:0]
	for _, ce := range b.cellErrors {
		if ce.RowIndex != rowIndex || ce.ColName != colName {
			filtered = append(filtered, ce)
		}
	}
	b.cellErrors = filtered
	if ce, bad := validateCellISO88592(rowIndex, colName, value); bad {
		b.cellErrors = append(b.cellErrors, ce)
	}
	return b.buildResult()
}

// GetTableData returns the current in-memory table.
func (b *Booking) GetTableData() TableDataResult {
	return b.buildResult()
}

// ExportToExcel saves the current rows to an xlsx file chosen via save dialog.
func (b *Booking) ExportToExcel() error {
	defaultName := "kutyaguru_ExcelExport_" + time.Now().Format("20060102_150405") + ".xlsx"
	path, err := runtime.SaveFileDialog(b.ctx, runtime.SaveDialogOptions{
		Title:           "Munkaadatok mentése Excel fájlba",
		DefaultFilename: defaultName,
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel fájlok (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	// Write header row.
	for ci, name := range b.columnNames {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		f.SetCellValue(sheet, cell, name)
	}
	// Write data rows.
	for ri, row := range b.rows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	return f.SaveAs(path)
}

// ImportFromExcel loads rows from a previously exported xlsx file.
func (b *Booking) ImportFromExcel() (TableDataResult, error) {
	path, err := runtime.OpenFileDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "Munkaadatok betöltése Excel fájlból",
		Filters: []runtime.FileFilter{
			{DisplayName: "Excel fájlok (*.xlsx)", Pattern: "*.xlsx"},
		},
	})
	if err != nil || path == "" {
		return b.buildResult(), err
	}

	f, err := excelize.OpenFile(path, excelize.Options{RawCellValue: false})
	if err != nil {
		return TableDataResult{}, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return TableDataResult{}, fmt.Errorf("üres munkafüzet")
	}
	allRows, err := f.GetRows(sheets[0])
	if err != nil || len(allRows) == 0 {
		return TableDataResult{}, fmt.Errorf("munkalap olvasási hiba: %w", err)
	}

	// Row 0 is the header (column names).
	importCols := allRows[0]
	// Re-derive column order from what's in the file; missing fields get empty values.
	importIndex := make(map[string]int, len(importCols))
	for i, c := range importCols {
		importIndex[c] = i
	}

	b.rows = make([][]string, 0, len(allRows)-1)
	for _, dataRow := range allRows[1:] {
		row := make([]string, len(b.columnNames))
		for ci, name := range b.columnNames {
			if idx, ok := importIndex[name]; ok && idx < len(dataRow) {
				row[ci] = dataRow[idx]
			}
		}
		b.rows = append(b.rows, row)
	}
	b.cellErrors = b.validateAllCells()
	return b.buildResult(), nil
}

// SaveCSV writes the Számlázz.hu CSV to a user-chosen file.
// encoding must be "ISO-8859-2" or "UTF-8".
func (b *Booking) SaveCSV(encoding string) error {
	if len(b.cellErrors) > 0 {
		ce := b.cellErrors[0]
		return fmt.Errorf("nem menthető: a %d. sor %q oszlopában nem kódolható karakter: %q (pozíció: %d)",
			ce.RowIndex+1, ce.ColName, ce.InvalidChar, ce.CharPos)
	}

	path, err := runtime.SaveFileDialog(b.ctx, runtime.SaveDialogOptions{
		Title: "CSV mentése (Számlázz.hu)",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV fájlok (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil || path == "" {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("fájl létrehozási hiba: %w", err)
	}
	defer file.Close()

	var w io.Writer = file
	if strings.EqualFold(encoding, "ISO-8859-2") {
		w = charmap.ISO8859_2.NewEncoder().Writer(file)
	}

	return b.writeCSV(w)
}

// GetStatus returns the currently open Excel file path for the status bar.
func (b *Booking) GetStatus() string {
	return b.excelPath
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func (b *Booking) buildResult() TableDataResult {
	cols := make([]string, len(b.columnNames))
	copy(cols, b.columnNames)
	rows := make([][]string, len(b.rows))
	for i, r := range b.rows {
		rows[i] = make([]string, len(r))
		copy(rows[i], r)
	}
	errs := make([]CellError, len(b.cellErrors))
	copy(errs, b.cellErrors)
	return TableDataResult{Columns: cols, Rows: rows, CellErrors: errs}
}

func (b *Booking) getCellByColName(row []string, colName string) string {
	for i, name := range b.columnNames {
		if name == colName && i < len(row) {
			return row[i]
		}
	}
	return ""
}

func (b *Booking) validateAllCells() []CellError {
	var errs []CellError
	for ri, row := range b.rows {
		for ci, val := range row {
			if ce, bad := validateCellISO88592(ri, b.columnNames[ci], val); bad {
				errs = append(errs, ce)
			}
		}
	}
	return errs
}

func (b *Booking) writeCSV(w io.Writer) error {
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for _, line := range b.tmpl.HeaderLines {
		fmt.Fprintln(bw, line)
	}
	for _, colDef := range b.tmpl.ColDefLines {
		fmt.Fprintln(bw, strings.Join(colDef, ";"))
	}
	for i, row := range b.rows {
		num := i + 1
		fmt.Fprintf(bw, "%d", num)
		for _, colDef := range b.tmpl.ColDefLines {
			for _, colName := range colDef {
				cell := b.getCellByColName(row, colName)
				fmt.Fprintf(bw, "%s;", cell)
			}
			fmt.Fprintln(bw)
		}
	}
	return nil
}

// ─── Standalone functions ─────────────────────────────────────────────────────

// validateCellISO88592 returns a CellError and true if any rune in value
// cannot be encoded in ISO-8859-2.
func validateCellISO88592(rowIndex int, colName, value string) (CellError, bool) {
	for pos, r := range value {
		if _, ok := charmap.ISO8859_2.EncodeRune(r); !ok {
			return CellError{
				RowIndex:    rowIndex,
				ColName:     colName,
				Value:       value,
				InvalidChar: string(r),
				CharPos:     pos,
			}, true
		}
	}
	return CellError{}, false
}

// loadTemplate parses the embedded basic_sablon.csv (ISO-8859-2 encoded).
func loadTemplate(data []byte) (templateData, error) {
	decoded, err := charmap.ISO8859_2.NewDecoder().Bytes(data)
	if err != nil {
		return templateData{}, fmt.Errorf("template dekódolási hiba: %w", err)
	}

	var tmpl templateData
	scanner := bufio.NewScanner(bytes.NewReader(decoded))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, ";") {
			break
		}
		if strings.HasPrefix(line, ";;") {
			tmpl.HeaderLines = append(tmpl.HeaderLines, line)
		} else {
			tmpl.ColDefLines = append(tmpl.ColDefLines, strings.Split(line, ";"))
		}
	}
	return tmpl, scanner.Err()
}

// parseFieldsYAML parses fields.yaml using yaml.Node to preserve key order.
func parseFieldsYAML(data []byte) ([]Field, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("üres YAML fájl")
	}
	root := doc.Content[0]

	findKey := func(node *yaml.Node, key string) *yaml.Node {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1]
			}
		}
		return nil
	}

	var fields []Field

	if mappingsNode := findKey(root, "mappings"); mappingsNode != nil {
		for i := 0; i+1 < len(mappingsNode.Content); i += 2 {
			fields = append(fields, Field{
				Name:    mappingsNode.Content[i].Value,
				Type:    FieldTypeMapping,
				Mapping: mappingsNode.Content[i+1].Value,
			})
		}
	}

	if constantsNode := findKey(root, "constants"); constantsNode != nil {
		for i := 0; i+1 < len(constantsNode.Content); i += 2 {
			fields = append(fields, Field{
				Name:  constantsNode.Content[i].Value,
				Type:  FieldTypeConst,
				Value: constantsNode.Content[i+1].Value,
			})
		}
	}

	if editablesNode := findKey(root, "editables"); editablesNode != nil {
		for i := 0; i+1 < len(editablesNode.Content); i += 2 {
			key := editablesNode.Content[i].Value
			var ef editableFieldYAML
			if err := editablesNode.Content[i+1].Decode(&ef); err != nil {
				return nil, fmt.Errorf("editable %q: %w", key, err)
			}
			f := Field{Name: key}
			if ef.Type == "date" {
				f.Type = FieldTypeDate
				t := time.Now().AddDate(0, 0, ef.Plus)
				f.Value = t.Format("2006.01.02")
			} else {
				f.Type = FieldTypeText
				f.Value = ef.Value
				f.Options = ef.Options
			}
			fields = append(fields, f)
		}
	}

	return fields, nil
}
