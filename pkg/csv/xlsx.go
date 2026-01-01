package csv

import (
	"encoding/csv"
	"io"

	"github.com/tealeg/xlsx/v3"
)

// TODO: cleanup; no support io interfaces ? :c
func FromXLSX(fileName string, w io.Writer) error {
	xlsxFile, err := xlsx.OpenFile(fileName)
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(w)

	sheet := xlsxFile.Sheets[0]
	var vals []string
	err = sheet.ForEachRow(func(row *xlsx.Row) error {
		if row == nil {
			return nil // TODO: ?
		}

		vals = vals[:0]
		err := row.ForEachCell(func(cell *xlsx.Cell) error {
			str, err := cell.FormattedValue()
			if err != nil {
				return err
			}

			vals = append(vals, str)
			return nil
		})
		if err != nil {
			return err
		}

		csvWriter.Write(vals)
		return nil
	})
	if err != nil {
		return err
	}

	csvWriter.Flush()
	return nil
}
