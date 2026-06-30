package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
)

const (
	fixtureSheet = "Foglalások"
	fixedKelt    = "2025.01.15"
	fixedTelj    = "2025.01.15"
	fixedHatar   = "2025.01.18"
)

// fixtureHeaders are the SOURCE column names in the Booked4us XLSX
// (the "mapping" values from fields.yaml, not the output field names).
var fixtureHeaders = []string{
	"Ügyfél neve", "Ügyfél e-mail címe", "Szolgáltatás",
	"Foglalások száma", "Irányító szám", "Város", "Utca, házszám",
}

// fixtureRows uses ASCII-only values so pandas and excelize agree on string representation.
// (pandas reads number-typed cells as float64, so "1" becomes "1.0"; SetCellStr avoids this.)
var fixtureRows = [][]string{
	{"Kovacs Rita", "kovacs@example.com", "Smart Puppy - Kolyok", "1", "1234", "Budapest", "Fo utca 1."},
	{"Toth Laszlo", "toth@example.com", "Smart Puppy - Felnott", "2", "5678", "Debrecen", "Kossuth ter 5."},
	{"Nagy Anna", "nagy@example.com", "Versenyfelkeszito", "1", "9012", "Gyor", "Arpad ut 12."},
}

func TestMain(m *testing.M) {
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		panic("mkdir testdata: " + err.Error())
	}
	if err := createFixtureXLSX("testdata/fixture.xlsx"); err != nil {
		panic("create fixture XLSX: " + err.Error())
	}
	os.Exit(m.Run())
}

func createFixtureXLSX(path string) error {
	f := excelize.NewFile()
	defer f.Close()
	idx, _ := f.NewSheet(fixtureSheet)
	f.SetActiveSheet(idx)
	f.DeleteSheet("Sheet1")
	for ci, h := range fixtureHeaders {
		cell, _ := excelize.CoordinatesToCellName(ci+1, 1)
		f.SetCellStr(fixtureSheet, cell, h)
	}
	for ri, row := range fixtureRows {
		for ci, val := range row {
			cell, _ := excelize.CoordinatesToCellName(ci+1, ri+2)
			f.SetCellStr(fixtureSheet, cell, val)
		}
	}
	return f.SaveAs(path)
}

func newTestBooking(t *testing.T) *Booking {
	t.Helper()
	b := newBooking()
	b.store = &settingsStore{path: filepath.Join(t.TempDir(), "settings.json")}
	b.settings = defaultSettings()

	fields, err := parseFieldsYAML(defaultFieldsYAML)
	if err != nil {
		t.Fatal(err)
	}
	b.fields = fields
	b.columnNames = make([]string, len(fields))
	for i, f := range fields {
		b.columnNames[i] = f.Name
	}

	tmpl, err := loadTemplate(templateCSVBytes)
	if err != nil {
		t.Fatal(err)
	}
	b.tmpl = tmpl

	// Fix date fields so output is deterministic regardless of when the test runs.
	b.UpdateFieldValue("Kelt", fixedKelt)
	b.UpdateFieldValue("Teljesítés", fixedTelj)
	b.UpdateFieldValue("Fizetési határidő", fixedHatar)

	return b
}

// TestCSVMatchesPythonGolden verifies byte-identical output with the Python implementation.
// Generate the golden file first: python3 testdata/generate_golden.py
func TestCSVMatchesPythonGolden(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	var buf bytes.Buffer
	if err := b.writeCSV(charmap.ISO8859_2.NewEncoder().Writer(&buf)); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	golden, err := os.ReadFile("testdata/golden.csv")
	if err != nil {
		t.Skipf("golden file not yet generated — run: python3 testdata/generate_golden.py\n(%v)", err)
	}

	if bytes.Equal(buf.Bytes(), golden) {
		return
	}

	t.Error("Go CSV output differs from Python golden")
	got := bytes.Split(buf.Bytes(), []byte("\n"))
	want := bytes.Split(golden, []byte("\n"))
	n := max(len(got), len(want))
	for i := range n {
		var g, w []byte
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if !bytes.Equal(g, w) {
			t.Errorf("  line %d\n    got:  %q\n    want: %q", i+1, g, w)
		}
	}
}

// TestCSVRowCount verifies the number of data rows in the CSV matches the fixture.
func TestCSVRowCount(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}
	if len(b.rows) != len(fixtureRows) {
		t.Errorf("row count: got %d, want %d", len(b.rows), len(fixtureRows))
	}
}

// TestCSVMappedValues verifies that MAPPING fields are read from the correct source columns.
func TestCSVMappedValues(t *testing.T) {
	b := newTestBooking(t)
	b.excelPath = "testdata/fixture.xlsx"
	if _, err := b.LoadSheet(fixtureSheet); err != nil {
		t.Fatalf("LoadSheet: %v", err)
	}

	// "Partner megnevezése:" is mapped from "Ügyfél neve"
	partnerIdx := -1
	for i, name := range b.columnNames {
		if name == "Partner megnevezése:" {
			partnerIdx = i
			break
		}
	}
	if partnerIdx == -1 {
		t.Fatal("column 'Partner megnevezése:' not found")
	}
	for ri, row := range b.rows {
		want := fixtureRows[ri][0] // "Ügyfél neve" is fixtureHeaders[0]
		if got := row[partnerIdx]; got != want {
			t.Errorf("row %d Partner megnevezése: got %q, want %q", ri, got, want)
		}
	}
}
