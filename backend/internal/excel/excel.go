package excel

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// ExportToExcel exports a slice of maps representing rows to the provided writer as an XLSX file.
// headers contains the keys/headers to be used, and they will be written as the first row.
func ExportToExcel(sheetName string, headers []string, data []map[string]interface{}, w io.Writer) error {
	f := excelize.NewFile()
	defer f.Close()

	index, err := f.NewSheet(sheetName)
	if err != nil {
		return err
	}
	f.SetActiveSheet(index)

	// Delete default sheet if it's different
	if sheetName != "Sheet1" {
		f.DeleteSheet("Sheet1")
	}

	// Write headers
	for colIdx, h := range headers {
		colName, err := excelize.ColumnNumberToName(colIdx + 1)
		if err != nil {
			return err
		}
		cell := fmt.Sprintf("%s1", colName)
		f.SetCellValue(sheetName, cell, h)
	}

	// Write data rows
	for rowIdx, rowData := range data {
		for colIdx, h := range headers {
			colName, err := excelize.ColumnNumberToName(colIdx + 1)
			if err != nil {
				return err
			}
			cell := fmt.Sprintf("%s%d", colName, rowIdx+2)
			f.SetCellValue(sheetName, cell, rowData[h])
		}
	}

	return f.Write(w)
}

// ParseExcel parses rows from the given reader's first sheet or specified sheet.
// It returns a list of map records using the header names from the first row.
func ParseExcel(r io.Reader, sheetName string) ([]map[string]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if sheetName == "" {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, fmt.Errorf("no sheets found in the excel file")
		}
		sheetName = sheets[0]
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if len(rows) < 1 {
		return nil, fmt.Errorf("excel file is empty")
	}

	headers := rows[0]
	var data []map[string]string

	for rowIdx := 1; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		rowData := make(map[string]string)
		for colIdx, val := range row {
			if colIdx < len(headers) {
				rowData[headers[colIdx]] = val
			}
		}
		// Only add non-empty rows
		isEmpty := true
		for _, val := range rowData {
			if val != "" {
				isEmpty = false
				break
			}
		}
		if !isEmpty {
			data = append(data, rowData)
		}
	}

	return data, nil
}
