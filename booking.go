package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
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

// Output column names referenced by validation and per-service pricing.
const (
	colService  = "Tétel megnevezés"     // service / item description
	colPrice    = "Nettó egységár"       // net unit price
	colPartner  = "Partner megnevezése:" // partner (customer) name
	colEmail    = "Email"
	colPostal   = "Irányítószám"
	colQuantity = "Mennyiség"
)

// CellError severities (also used as the CSS class selector on the frontend).
const (
	severityError   = "error"   // red: blocks export (encoding cannot represent the char)
	severityMapped  = "mapped"  // yellow: will be substituted on export
	severityWarning = "warning" // orange: content quality issue, never blocks export
)

const maxRecentFiles = 10

// Settings holds user preferences persisted across sessions.
type Settings struct {
	ColorScheme string            `json:"colorScheme"` // "light" | "dark" | "auto"
	Encoding    string            `json:"encoding"`    // "ISO-8859-2" | "UTF-8"
	CharMapping map[string]string `json:"charMapping"` // unicode char → latin-2 replacement
	FieldValues map[string]string `json:"fieldValues"` // persisted TEXT editable field values

	ServicePrices map[string]string `json:"servicePrices"` // service name → net unit price
	RecentFiles   []string          `json:"recentFiles"`   // most-recent-first, capped
	WindowX       int               `json:"windowX"`
	WindowY       int               `json:"windowY"`
	WindowW       int               `json:"windowW"`
	WindowH       int               `json:"windowH"`
}

func defaultSettings() Settings {
	return Settings{
		ColorScheme:   "auto",
		Encoding:      "ISO-8859-2",
		CharMapping:   map[string]string{},
		FieldValues:   map[string]string{},
		ServicePrices: map[string]string{},
		WindowW:       1280,
		WindowH:       800,
	}
}

// Field is serialised to JSON and sent to the frontend for the Mezők tab.
type Field struct {
	Name    string    `json:"name"`
	Type    FieldType `json:"type"`
	Mapping string    `json:"mapping,omitempty"`
	Value   string    `json:"value"`
	Options []string  `json:"options,omitempty"`
}

// CellError describes one cell whose value cannot be encoded in the current encoding.
type CellError struct {
	RowIndex    int    `json:"rowIndex"`
	ColName     string `json:"colName"`
	Value       string `json:"value"`
	InvalidChar string `json:"invalidChar"`
	CharPos     int    `json:"charPos"`
	Mapped      bool   `json:"mapped"`   // true → substitution exists (yellow); false → blocked (red)
	MappedTo    string `json:"mappedTo"` // replacement string when Mapped==true
	Severity    string `json:"severity"` // "error" (red) | "mapped" (yellow) | "warning" (orange)
	Message     string `json:"message"`  // human-readable description for tooltip / status bar
}

// TableDataResult is returned to the frontend whenever the table changes.
type TableDataResult struct {
	Columns    []string    `json:"columns"`
	Rows       [][]string  `json:"rows"`
	CellErrors []CellError `json:"cellErrors"`
}

// templateData holds the parsed Számlázz.hu CSV template.
type templateData struct {
	HeaderLines []string
	ColDefLines [][]string
}

type editableFieldYAML struct {
	Type    string   `yaml:"type"`
	Value   string   `yaml:"value"`
	Options []string `yaml:"options"`
	Today   bool     `yaml:"today"`
	Plus    int      `yaml:"plus"`
}

// Booking is the struct bound to Wails (registered as a v3 service).
type Booking struct {
	app          *application.App
	win          *application.WebviewWindow
	fields       []Field
	columnNames  []string
	rows         [][]string
	tmpl         templateData
	excelPath    string
	cellErrors   []CellError
	settings     Settings
	settingsPath string
}

func newBooking() *Booking { return &Booking{} }

func (b *Booking) init() error {
	// Determine settings file path.
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		cfgDir = os.TempDir()
	}
	b.settingsPath = filepath.Join(cfgDir, "kutyaguru", "settings.json")

	// Load persisted settings (falls back to defaults on any error).
	b.settings = b.loadSettings()

	fields, err := parseFieldsYAML(defaultFieldsYAML)
	if err != nil {
		return fmt.Errorf("fields.yaml: %w", err)
	}
	b.fields = fields
	b.restoreFieldValues()
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

// restoreFieldValues applies persisted editable (TEXT) values onto the parsed
// fields. DATE fields are intentionally left computed-from-today so the dates
// always reflect the current run.
func (b *Booking) restoreFieldValues() {
	for i := range b.fields {
		if b.fields[i].Type != FieldTypeText {
			continue
		}
		if v, ok := b.settings.FieldValues[b.fields[i].Name]; ok {
			b.fields[i].Value = v
		}
	}
}

func (b *Booking) loadSettings() Settings {
	s := defaultSettings()
	data, err := os.ReadFile(b.settingsPath)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings()
	}
	// Ensure non-zero window size after loading.
	if s.WindowW == 0 {
		s.WindowW = 1280
	}
	if s.WindowH == 0 {
		s.WindowH = 800
	}
	if s.CharMapping == nil {
		s.CharMapping = map[string]string{}
	}
	if s.FieldValues == nil {
		s.FieldValues = map[string]string{}
	}
	if s.ServicePrices == nil {
		s.ServicePrices = map[string]string{}
	}
	return s
}

func (b *Booking) saveSettings() {
	data, err := json.MarshalIndent(b.settings, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(b.settingsPath), 0o755)
	_ = os.WriteFile(b.settingsPath, data, 0o644)
}

// updateGeometry copies the live window position/size into the in-memory
// settings. It does NOT touch disk — it is called on every move/resize, so the
// flush is deferred to WindowClosing / ServiceShutdown to avoid disk thrash.
func (b *Booking) updateGeometry() {
	if b.win == nil {
		return
	}
	x, y := b.win.Position()
	w, h := b.win.Size()
	b.settings.WindowX, b.settings.WindowY = x, y
	b.settings.WindowW, b.settings.WindowH = w, h
}

// ServiceShutdown flushes the (already-fresh) settings to disk when the app
// quits. It deliberately does not re-read the window — on the close-button path
// the window may already be tearing down, and move/resize have kept geometry
// current. This is the backstop for the macOS Cmd+Q path, which can skip the
// per-window WindowClosing hook.
func (b *Booking) ServiceShutdown() error {
	b.saveSettings()
	return nil
}

// ─── Wails-exposed methods ────────────────────────────────────────────────────

// GetSettings returns the current user settings (called on frontend mount).
func (b *Booking) GetSettings() Settings {
	return b.settings
}

// SaveSettings persists the given settings to disk (called when user changes theme/encoding).
func (b *Booking) SaveSettings(s Settings) error {
	b.settings = s
	b.saveSettings()
	return nil
}

// SetColorScheme persists the UI color scheme ("light" | "dark" | "auto").
func (b *Booking) SetColorScheme(scheme string) {
	b.settings.ColorScheme = scheme
	b.saveSettings()
}

// SetEncoding updates the CSV encoding, re-validates all cells, persists the
// choice, and returns the new table state.
func (b *Booking) SetEncoding(enc string) TableDataResult {
	b.settings.Encoding = enc
	b.saveSettings()
	b.cellErrors = b.validateAllCells()
	return b.buildResult()
}

// GetCharMapping returns the current unicode→replacement substitution map.
func (b *Booking) GetCharMapping() map[string]string {
	return b.settings.CharMapping
}

// SetCharMapping replaces the substitution map, re-validates all cells, saves settings.
func (b *Booking) SetCharMapping(m map[string]string) TableDataResult {
	b.settings.CharMapping = m
	b.saveSettings()
	b.cellErrors = b.validateAllCells()
	return b.buildResult()
}

// GetServicePrices returns the current service → net-unit-price lookup.
func (b *Booking) GetServicePrices() map[string]string {
	return b.settings.ServicePrices
}

// SetServicePrices replaces the price lookup, re-applies it to all rows,
// re-validates, and saves settings.
func (b *Booking) SetServicePrices(m map[string]string) TableDataResult {
	b.settings.ServicePrices = m
	b.saveSettings()
	b.applyServicePrices()
	b.cellErrors = b.validateAllCells()
	return b.buildResult()
}

// OpenBookedFile shows a file-open dialog for xlsx files and returns the sheet names.
func (b *Booking) OpenBookedFile() ([]string, error) {
	path, err := b.app.Dialog.OpenFile().
		SetTitle("Booked4us Excel megnyitása").
		AddFilter("Excel fájlok (*.xlsx)", "*.xlsx").
		AttachToWindow(b.win).
		PromptForSingleSelection()
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("nem sikerült megnyitni: %w", err)
	}
	defer f.Close()

	b.excelPath = path
	b.pushRecent(path)
	return f.GetSheetList(), nil
}

// GetRecentFiles returns the most-recently-opened file paths (newest first).
func (b *Booking) GetRecentFiles() []string {
	return b.settings.RecentFiles
}

// LoadRecentFile reopens a previously used file without a dialog and returns its
// sheet names. A missing file is dropped from the recent list and reported.
func (b *Booking) LoadRecentFile(path string) ([]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		b.removeRecent(path)
		return nil, fmt.Errorf("a fájl nem nyitható meg: %w", err)
	}
	defer f.Close()

	b.excelPath = path
	b.pushRecent(path)
	return f.GetSheetList(), nil
}

// pushRecent moves path to the front of the recent list (deduped, capped).
func (b *Booking) pushRecent(path string) {
	list := make([]string, 0, maxRecentFiles)
	list = append(list, path)
	for _, p := range b.settings.RecentFiles {
		if p != path {
			list = append(list, p)
		}
	}
	if len(list) > maxRecentFiles {
		list = list[:maxRecentFiles]
	}
	b.settings.RecentFiles = list
	b.saveSettings()
}

// removeRecent drops path from the recent list.
func (b *Booking) removeRecent(path string) {
	list := b.settings.RecentFiles[:0]
	for _, p := range b.settings.RecentFiles {
		if p != path {
			list = append(list, p)
		}
	}
	b.settings.RecentFiles = list
	b.saveSettings()
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

	b.applyServicePrices()
	b.cellErrors = b.validateAllCells()
	return b.buildResult(), nil
}

// GetFields returns the current field definitions for the Mezők tab.
func (b *Booking) GetFields() []Field {
	return b.fields
}

// UpdateFieldValue mutates the in-memory value of an editable field.
func (b *Booking) UpdateFieldValue(fieldName, value string) error {
	for i := range b.fields {
		if b.fields[i].Name == fieldName {
			if b.fields[i].Type == FieldTypeMapping || b.fields[i].Type == FieldTypeConst {
				return fmt.Errorf("%q mező nem szerkeszthető", fieldName)
			}
			b.fields[i].Value = value
			// Persist editable (TEXT) values so they survive restarts; DATE
			// fields stay transient (recomputed from today on each launch).
			if b.fields[i].Type == FieldTypeText {
				if b.settings.FieldValues == nil {
					b.settings.FieldValues = map[string]string{}
				}
				b.settings.FieldValues[fieldName] = value
				b.saveSettings()
			}
			return nil
		}
	}
	return fmt.Errorf("ismeretlen mező: %q", fieldName)
}

// ReapplyFields re-applies current editable/const/date field values to all non-MAPPING rows.
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
	b.applyServicePrices()
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
	// Full revalidation keeps content and unpriced-service warnings consistent
	// when editing a service or price cell affects another cell. Tables are
	// small (tens of rows), so the cost is negligible.
	b.cellErrors = b.validateAllCells()
	return b.buildResult()
}

// GetTableData returns the current in-memory table.
func (b *Booking) GetTableData() TableDataResult {
	return b.buildResult()
}

// ExportToExcel saves the current rows to an xlsx file chosen via save dialog.
// It returns true when a file was written, false when the user cancelled.
func (b *Booking) ExportToExcel() (bool, error) {
	defaultName := "kutyaguru_ExcelExport_" + time.Now().Format("20060102_150405") + ".xlsx"
	path, err := b.app.Dialog.SaveFile().
		SetFilename(defaultName).
		AddFilter("Excel fájlok (*.xlsx)", "*.xlsx").
		AttachToWindow(b.win).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	for ci, name := range b.columnNames {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		f.SetCellValue(sheet, cell, name)
	}
	for ri, row := range b.rows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellValue(sheet, cell, val)
		}
	}
	if err := f.SaveAs(path); err != nil {
		return false, err
	}
	return true, nil
}

// ImportFromExcel loads rows from a previously exported xlsx file.
func (b *Booking) ImportFromExcel() (TableDataResult, error) {
	path, err := b.app.Dialog.OpenFile().
		SetTitle("Munkaadatok betöltése Excel fájlból").
		AddFilter("Excel fájlok (*.xlsx)", "*.xlsx").
		AttachToWindow(b.win).
		PromptForSingleSelection()
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

	importCols := allRows[0]
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

// SaveCSV writes the Számlázz.hu CSV using the encoding from settings.
// SaveCSV returns true when a file was written, false when the user cancelled
// the dialog (so the frontend can skip the success notification).
func (b *Booking) SaveCSV() (bool, error) {
	for _, ce := range b.cellErrors {
		if ce.Severity == severityError {
			return false, fmt.Errorf("nem menthető: a %d. sor %q oszlopában nem kódolható karakter: %q (pozíció: %d)",
				ce.RowIndex+1, ce.ColName, ce.InvalidChar, ce.CharPos)
		}
	}

	path, err := b.app.Dialog.SaveFile().
		SetFilename("szamlazz.csv").
		AddFilter("CSV fájlok (*.csv)", "*.csv").
		AttachToWindow(b.win).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}

	file, err := os.Create(path)
	if err != nil {
		return false, fmt.Errorf("fájl létrehozási hiba: %w", err)
	}
	defer file.Close()

	var w io.Writer = file
	if strings.EqualFold(b.settings.Encoding, "ISO-8859-2") {
		w = charmap.ISO8859_2.NewEncoder().Writer(file)
	}

	if err := b.writeCSV(w); err != nil {
		return false, err
	}
	return true, nil
}

// PreviewCSV renders the CSV exactly as SaveCSV would (char mapping applied) but
// returns it as a UTF-8 string for on-screen display, regardless of the export
// encoding.
func (b *Booking) PreviewCSV() (string, error) {
	var buf bytes.Buffer
	if err := b.writeCSV(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
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

// colIndex returns the column index for the given output column name, or -1.
func (b *Booking) colIndex(colName string) int {
	for i, name := range b.columnNames {
		if name == colName {
			return i
		}
	}
	return -1
}

// applyServicePrices overwrites each row's net-unit-price cell with the price
// configured for that row's service, when one exists. Rows whose service has no
// configured price keep whatever value is already there (the flat default).
func (b *Booking) applyServicePrices() {
	if len(b.settings.ServicePrices) == 0 {
		return
	}
	svcIdx := b.colIndex(colService)
	priceIdx := b.colIndex(colPrice)
	if svcIdx < 0 || priceIdx < 0 {
		return
	}
	for ri := range b.rows {
		if svcIdx >= len(b.rows[ri]) || priceIdx >= len(b.rows[ri]) {
			continue
		}
		if price, ok := b.settings.ServicePrices[b.rows[ri][svcIdx]]; ok {
			b.rows[ri][priceIdx] = price
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

// validateAllCells produces one CellError per problematic cell. Encoding checks
// run only outside UTF-8 mode; content-quality and unpriced-service warnings run
// always (content validity is encoding-independent).
func (b *Booking) validateAllCells() []CellError {
	checkEncoding := !strings.EqualFold(b.settings.Encoding, "UTF-8")
	svcIdx := b.colIndex(colService)
	priceIdx := b.colIndex(colPrice)

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

	for ri, row := range b.rows {
		for ci, val := range row {
			colName := b.columnNames[ci]
			if checkEncoding {
				consider(validateCell(ri, colName, val, b.settings.CharMapping))
			}
			consider(validateContent(ri, colName, val))
			// Unpriced-service warning: the net-unit-price cell of a row whose
			// service has no configured price.
			if ci == priceIdx && svcIdx >= 0 && svcIdx < len(row) {
				svc := row[svcIdx]
				if _, priced := b.settings.ServicePrices[svc]; !priced && strings.TrimSpace(svc) != "" {
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

// applyMapping substitutes characters in value using the char mapping.
func (b *Booking) applyMapping(value string) string {
	if len(b.settings.CharMapping) == 0 {
		return value
	}
	var sb strings.Builder
	for _, r := range value {
		if repl, ok := b.settings.CharMapping[string(r)]; ok {
			sb.WriteString(repl)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
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
				cell := b.applyMapping(b.getCellByColName(row, colName))
				fmt.Fprintf(bw, "%s;", cell)
			}
			fmt.Fprintln(bw)
		}
	}
	return nil
}

// ─── Standalone functions ─────────────────────────────────────────────────────

// validateCell checks a single cell value against ISO-8859-2, consulting the char mapping.
// Unmapped invalid chars (red) take priority over mapped ones (yellow).
func validateCell(rowIndex int, colName, value string, mapping map[string]string) (CellError, bool) {
	var firstMapped *CellError
	for pos, r := range value {
		if _, ok := charmap.ISO8859_2.EncodeRune(r); ok {
			continue
		}
		char := string(r)
		if repl, inMap := mapping[char]; inMap {
			if firstMapped == nil {
				ce := CellError{RowIndex: rowIndex, ColName: colName, Value: value,
					InvalidChar: char, CharPos: pos, Mapped: true, MappedTo: repl,
					Severity: severityMapped,
					Message:  fmt.Sprintf("Exportáláskor helyettesítve: %q → %q", char, repl)}
				firstMapped = &ce
			}
		} else {
			return CellError{RowIndex: rowIndex, ColName: colName, Value: value,
				InvalidChar: char, CharPos: pos, Mapped: false,
				Severity: severityError,
				Message:  fmt.Sprintf("Nem kódolható karakter: %q", char)}, true
		}
	}
	if firstMapped != nil {
		return *firstMapped, true
	}
	return CellError{}, false
}

// emailRe is a deliberately lenient address@host.tld check — enough to catch
// obvious typos without rejecting unusual but valid addresses.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// validateContent returns a content-quality warning for a cell, keyed by its
// output column name. These never block export; they only surface as orange.
func validateContent(rowIndex int, colName, value string) (CellError, bool) {
	warn := func(msg string) (CellError, bool) {
		return CellError{RowIndex: rowIndex, ColName: colName, Value: value,
			Severity: severityWarning, Message: msg}, true
	}
	switch colName {
	case colPartner:
		if strings.TrimSpace(value) == "" {
			return warn("Hiányzó partner név")
		}
	case colEmail:
		if strings.TrimSpace(value) == "" {
			return warn("Hiányzó e-mail cím")
		}
		if !emailRe.MatchString(strings.TrimSpace(value)) {
			return warn("Érvénytelen e-mail cím")
		}
	case colPostal:
		v := strings.TrimSpace(value)
		if len(v) != 4 || strings.IndexFunc(v, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return warn("Az irányítószám 4 számjegyű")
		}
	case colQuantity:
		v := strings.TrimSpace(value)
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return warn("Érvénytelen mennyiség")
		}
	}
	return CellError{}, false
}

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
