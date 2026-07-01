package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
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

// colEnabledHeader is the extra column appended to Excel exports that records
// each row's on/off state ("1"/"0"), so an export → re-import round-trip
// restores the toggles. Plain Booked4us files lack it ⇒ all rows enabled.
const colEnabledHeader = "_Aktív"

// CellError severities (also used as the CSS class selector on the frontend).
const (
	severityError   = "error"   // red: blocks export (encoding cannot represent the char)
	severityMapped  = "mapped"  // yellow: will be substituted on export
	severityWarning = "warning" // orange: content quality issue, never blocks export
)

const maxRecentFiles = 10

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
	RowEnabled []bool      `json:"rowEnabled"`
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

// Booking is the struct bound to Wails (registered as a v3 service). It owns the
// application-level concerns (field schema, window, settings, Excel/dialog I/O)
// and delegates all table state and operations to doc.
type Booking struct {
	app       *application.App
	win       *application.WebviewWindow
	fields    []Field
	doc       document
	tmpl      templateData
	excelPath string

	// mu guards settings. It is contended by construction: the native window
	// event loop mutates settings.Window* on move/resize/close, while Wails
	// dispatches every bound service call on its own net/http goroutine — so a
	// geometry write can race the whole-struct marshal in saveSettings, or a
	// setter replacing one of the maps. mu serialises those. It does NOT cover
	// doc: doc is reached only through bound calls, which the UI issues one at a
	// time, so it has no concurrent-by-construction writer. For the same reason a
	// few non-Window settings reads on bound-call goroutines (SaveCSV, LoadSheet,
	// PreviewCSV) read the maps without mu: the geometry loop never writes those
	// fields, and the setters that do are UI-serialised against them, same as doc.
	mu       sync.Mutex
	settings Settings
	store    *settingsStore
}

func newBooking() *Booking { return &Booking{store: newSettingsStore()} }

func (b *Booking) init() error {
	// Load persisted settings (falls back to defaults on any error).
	b.settings = b.store.load()

	fields, err := parseFieldsYAML(defaultFieldsYAML)
	if err != nil {
		return fmt.Errorf("fields.yaml: %w", err)
	}
	b.fields = fields
	b.restoreFieldValues()
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = f.Name
	}
	b.doc.setColumns(names)

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

// saveSettings flushes the live in-memory settings to disk via the store,
// holding mu across the marshal so it cannot observe a half-written struct.
func (b *Booking) saveSettings() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.persist()
}

// persist flushes settings to disk without locking. The caller MUST already
// hold mu; it exists so the map-mutating setters can do "mutate + flush" as one
// critical section without re-locking (mu is not reentrant).
func (b *Booking) persist() {
	b.store.save(b.settings)
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
	b.mu.Lock()
	b.settings.WindowX, b.settings.WindowY = x, y
	b.settings.WindowW, b.settings.WindowH = w, h
	b.mu.Unlock()
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
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.settings
}

// SetColorScheme persists the UI color scheme ("light" | "dark" | "auto").
func (b *Booking) SetColorScheme(scheme string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.settings.ColorScheme = scheme
	b.persist()
}

// SetEncoding updates the CSV encoding, re-validates all cells, persists the
// choice, and returns the new table state.
func (b *Booking) SetEncoding(enc string) TableDataResult {
	b.mu.Lock()
	b.settings.Encoding = enc
	b.persist()
	s := b.settings
	b.mu.Unlock()

	b.doc.cellErrors = b.doc.validate(s)
	return b.doc.buildResult()
}

// GetCharMapping returns the current unicode→replacement substitution map.
func (b *Booking) GetCharMapping() map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.settings.CharMapping
}

// SetCharMapping replaces the substitution map, re-validates all cells, saves settings.
func (b *Booking) SetCharMapping(m map[string]string) TableDataResult {
	b.mu.Lock()
	b.settings.CharMapping = m
	b.persist()
	s := b.settings
	b.mu.Unlock()

	b.doc.cellErrors = b.doc.validate(s)
	return b.doc.buildResult()
}

// GetServicePrices returns the current service → net-unit-price lookup.
func (b *Booking) GetServicePrices() map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.settings.ServicePrices
}

// SetServicePrices replaces the price lookup, re-applies it to all rows,
// re-validates, and saves settings.
func (b *Booking) SetServicePrices(m map[string]string) TableDataResult {
	b.mu.Lock()
	b.settings.ServicePrices = m
	b.persist()
	s := b.settings
	b.mu.Unlock()

	b.doc.applyServicePrices(s.ServicePrices)
	b.doc.cellErrors = b.doc.validate(s)
	return b.doc.buildResult()
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
	names, err := sheetNames(path)
	if err != nil {
		return nil, err
	}
	b.excelPath = path
	b.pushRecent(path)
	return names, nil
}

// GetRecentFiles returns the most-recently-opened file paths (newest first).
func (b *Booking) GetRecentFiles() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.settings.RecentFiles
}

// LoadRecentFile reopens a previously used file without a dialog and returns its
// sheet names. A missing file is dropped from the recent list and reported.
func (b *Booking) LoadRecentFile(path string) ([]string, error) {
	names, err := sheetNames(path)
	if err != nil {
		b.removeRecent(path)
		return nil, fmt.Errorf("a fájl nem nyitható meg: %w", err)
	}
	b.excelPath = path
	b.pushRecent(path)
	return names, nil
}

// pushRecent moves path to the front of the recent list (deduped, capped).
func (b *Booking) pushRecent(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
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
	b.persist()
}

// removeRecent drops path from the recent list.
func (b *Booking) removeRecent(path string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	list := make([]string, 0, len(b.settings.RecentFiles))
	for _, p := range b.settings.RecentFiles {
		if p != path {
			list = append(list, p)
		}
	}
	b.settings.RecentFiles = list
	b.persist()
}

// LoadSheet reads the named sheet from the already-opened Excel file.
func (b *Booking) LoadSheet(sheetName string) (TableDataResult, error) {
	if b.excelPath == "" {
		return TableDataResult{}, fmt.Errorf("nincs megnyitott Excel fájl")
	}
	rows, err := readBookedSheet(b.excelPath, sheetName, b.fields)
	if err != nil {
		return TableDataResult{}, err
	}
	b.doc.rows = rows
	b.doc.rowEnabled = newEnabledSlice(len(b.doc.rows))
	if len(b.doc.rows) == 0 {
		b.doc.cellErrors = nil
		return b.doc.buildResult(), nil
	}

	b.doc.applyServicePrices(b.settings.ServicePrices)
	b.doc.cellErrors = b.doc.validate(b.settings)
	return b.doc.buildResult(), nil
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
				b.mu.Lock()
				if b.settings.FieldValues == nil {
					b.settings.FieldValues = map[string]string{}
				}
				b.settings.FieldValues[fieldName] = value
				b.persist()
				b.mu.Unlock()
			}
			return nil
		}
	}
	return fmt.Errorf("ismeretlen mező: %q", fieldName)
}

// ReapplyFields re-applies current editable/const/date field values to all non-MAPPING rows.
func (b *Booking) ReapplyFields() TableDataResult {
	for ri := range b.doc.rows {
		for _, field := range b.fields {
			if field.Type == FieldTypeMapping {
				continue
			}
			if idx := b.doc.colIndex(field.Name); idx >= 0 {
				b.doc.rows[ri][idx] = field.Value
			}
		}
	}
	b.doc.applyServicePrices(b.settings.ServicePrices)
	b.doc.cellErrors = b.doc.validate(b.settings)
	return b.doc.buildResult()
}

// UpdateCell mutates one cell and re-validates it in place.
func (b *Booking) UpdateCell(rowIndex int, colName, value string) TableDataResult {
	b.doc.updateCell(rowIndex, colName, value)
	// Full revalidation keeps content and unpriced-service warnings consistent
	// when editing a service or price cell affects another cell. Tables are
	// small (tens of rows), so the cost is negligible.
	b.doc.cellErrors = b.doc.validate(b.settings)
	return b.doc.buildResult()
}

// SetRowEnabled toggles whether a single row is included in CSV export.
func (b *Booking) SetRowEnabled(rowIndex int, enabled bool) TableDataResult {
	b.doc.setRowEnabled(rowIndex, enabled)
	return b.doc.buildResult()
}

// SetAllRowsEnabled toggles every row on or off (select-all / select-none).
func (b *Booking) SetAllRowsEnabled(enabled bool) TableDataResult {
	b.doc.setAllRowsEnabled(enabled)
	return b.doc.buildResult()
}

// GetTableData returns the current in-memory table.
func (b *Booking) GetTableData() TableDataResult {
	return b.doc.buildResult()
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
	if err := b.writeExcelTo(path); err != nil {
		return false, err
	}
	return true, nil
}

// writeExcelTo writes the current table (rows plus the trailing on/off flag
// column) to an xlsx file. See writeExcelFile for the format details.
func (b *Booking) writeExcelTo(path string) error {
	return writeExcelFile(path, b.doc.columnNames, b.doc.rows, b.doc.rowEnabled)
}

// ImportFromExcel loads rows from a previously exported xlsx file.
func (b *Booking) ImportFromExcel() (TableDataResult, error) {
	path, err := b.app.Dialog.OpenFile().
		SetTitle("Munkaadatok betöltése Excel fájlból").
		AddFilter("Excel fájlok (*.xlsx)", "*.xlsx").
		AttachToWindow(b.win).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return b.doc.buildResult(), err
	}
	if err := b.readExcelFrom(path); err != nil {
		return TableDataResult{}, err
	}
	return b.doc.buildResult(), nil
}

// readExcelFrom replaces the current table from an xlsx file, then re-validates.
// See readExcelFile for how columns and the flag column are matched.
func (b *Booking) readExcelFrom(path string) error {
	rows, rowEnabled, err := readExcelFile(path, b.doc.columnNames)
	if err != nil {
		return err
	}
	b.doc.rows = rows
	b.doc.rowEnabled = rowEnabled
	b.doc.cellErrors = b.doc.validate(b.settings)
	return nil
}

// SaveCSV writes the Számlázz.hu CSV using the encoding from settings.
// SaveCSV returns true when a file was written, false when the user cancelled
// the dialog (so the frontend can skip the success notification).
func (b *Booking) SaveCSV() (bool, error) {
	if err := b.doc.blockingExportError(); err != nil {
		return false, err
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

	if err := b.doc.writeCSV(w, b.tmpl, b.settings.CharMapping); err != nil {
		return false, err
	}
	return true, nil
}

// PreviewCSV renders the CSV exactly as SaveCSV would (char mapping applied) but
// returns it as a UTF-8 string for on-screen display, regardless of the export
// encoding.
func (b *Booking) PreviewCSV() (string, error) {
	var buf bytes.Buffer
	if err := b.doc.writeCSV(&buf, b.tmpl, b.settings.CharMapping); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GetStatus returns the currently open Excel file path for the status bar.
func (b *Booking) GetStatus() string {
	return b.excelPath
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
