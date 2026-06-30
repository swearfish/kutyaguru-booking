#!/usr/bin/env python3
"""Generate testdata/golden.csv from testdata/fixture.xlsx using the Python converter.

Run from inside booking.go/:
    python3 testdata/generate_golden.py

The golden file is committed and used by TestCSVMatchesPythonGolden in booking_test.go.
"""
import sys
from pathlib import Path

HERE    = Path(__file__).parent          # booking.go/testdata/
GO_ROOT = HERE.parent                    # booking.go/
PY_ROOT = GO_ROOT.parent / 'booking.py' # ../booking.py/
sys.path.insert(0, str(PY_ROOT))

from converter.field import read_fields_file
from converter.template import read_template_file
from converter.booked4us_xlsx import read_booked4us_excel_sheet
from converter.szamlazz_csv import convert_and_save_csv
import pandas as pd

# Must match the constants in booking_test.go
SHEET       = 'Foglalások'
FIXED_DATES = {
    'Kelt':                '2025.01.15',
    'Teljesítés':          '2025.01.15',
    'Fizetési határidő':   '2025.01.18',
}

# Use the Go assets directory so both sides read the same files.
fields = read_fields_file(GO_ROOT / 'assets' / 'fields.yaml')
header, columns = read_template_file(GO_ROOT / 'assets' / 'basic_sablon.csv')

for name, val in FIXED_DATES.items():
    fields[name].value = val

xlsx = pd.ExcelFile(str(HERE / 'fixture.xlsx'))
data = read_booked4us_excel_sheet(xlsx, fields, sheet=SHEET)

out = HERE / 'golden.csv'
convert_and_save_csv(out, header, columns, data)
print(f'Written: {out}')
