package legacyimport

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	db "oms-backend/internal/db/generated"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/xuri/excelize/v2"
)

const MaxUploadSize = 25 << 20

var MappableFields = []string{"name", "phone", "phone2", "address", "product", "quantity", "cod_amount", "status", "courier", "location", "remarks", "date"}

type UploadedTable struct {
	Headers []string
	Rows    []map[string]string
	Preview []map[string]string
}

func ParseUploadedFile(name string, data []byte) (UploadedTable, error) {
	if len(data) == 0 || len(data) > MaxUploadSize {
		return UploadedTable{}, fmt.Errorf("file must be between 1 byte and %d MB", MaxUploadSize>>20)
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".csv":
		return parseUploadedCSV(data)
	case ".xlsx":
		return parseUploadedXLSX(data)
	default:
		return UploadedTable{}, fmt.Errorf("only CSV and XLSX files are supported")
	}
}

func parseUploadedCSV(data []byte) (UploadedTable, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) == 0 || len(rows[0]) == 0 {
		return UploadedTable{}, fmt.Errorf("file is not a valid CSV table")
	}
	return tableFromRecords(rows)
}

func parseUploadedXLSX(data []byte) (UploadedTable, error) {
	if _, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err != nil {
		return UploadedTable{}, fmt.Errorf("file is not a valid XLSX workbook")
	}
	file, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{RawCellValue: true})
	if err != nil {
		return UploadedTable{}, fmt.Errorf("file is not a valid XLSX workbook")
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return UploadedTable{}, fmt.Errorf("workbook has no worksheets")
	}
	sheet := sheets[0]
	rows, err := file.GetRows(sheet)
	if err != nil || len(rows) == 0 || len(rows[0]) == 0 {
		return UploadedTable{}, fmt.Errorf("worksheet is empty or invalid")
	}
	for rowIndex := 1; rowIndex <= len(rows); rowIndex++ {
		for colIndex := 1; colIndex <= len(rows[rowIndex-1]); colIndex++ {
			cell, cellErr := excelize.CoordinatesToCellName(colIndex, rowIndex)
			if cellErr != nil {
				return UploadedTable{}, fmt.Errorf("invalid worksheet cell")
			}
			formula, formulaErr := file.GetCellFormula(sheet, cell)
			if formulaErr != nil {
				return UploadedTable{}, fmt.Errorf("could not validate worksheet values")
			}
			if formula != "" {
				return UploadedTable{}, fmt.Errorf("formulas are not allowed")
			}
		}
	}
	return tableFromRecords(rows)
}

func tableFromRecords(records [][]string) (UploadedTable, error) {
	headers := make([]string, len(records[0]))
	seen := make(map[string]bool)
	for index, header := range records[0] {
		header = strings.TrimSpace(header)
		if header == "" || seen[strings.ToLower(header)] {
			return UploadedTable{}, fmt.Errorf("headers must be non-empty and unique")
		}
		seen[strings.ToLower(header)] = true
		headers[index] = header
	}
	tableRows := make([]map[string]string, 0, len(records)-1)
	for rowIndex, record := range records[1:] {
		row := make(map[string]string, len(headers))
		for index, header := range headers {
			value := ""
			if index < len(record) {
				value = strings.TrimSpace(record[index])
			}
			if strings.HasPrefix(value, "=") {
				return UploadedTable{}, fmt.Errorf("formula-like values are not allowed")
			}
			row[header] = value
		}
		row["__line"] = strconv.Itoa(rowIndex + 2)
		tableRows = append(tableRows, row)
	}
	preview := tableRows
	if len(preview) > 5 {
		preview = preview[:5]
	}
	return UploadedTable{Headers: headers, Rows: tableRows, Preview: preview}, nil
}

func MapUploadedRows(table UploadedTable, mapping map[string]string) ([]MappedRow, error) {
	for _, field := range []string{"name", "phone", "address", "product", "cod_amount"} {
		if strings.TrimSpace(mapping[field]) == "" {
			return nil, fmt.Errorf("mapping for %s is required", field)
		}
		if !containsHeader(table.Headers, mapping[field]) {
			return nil, fmt.Errorf("mapping for %s references an unknown header", field)
		}
	}
	rows := make([]MappedRow, 0, len(table.Rows))
	for _, values := range table.Rows {
		get := func(field string) string {
			header := mapping[field]
			return values[header]
		}
		quantity := int32(1)
		if value := get("quantity"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 32)
			if err != nil || parsed < 1 {
				quantity = 0
			} else {
				quantity = int32(parsed)
			}
		}
		date := get("date")
		if date == "" {
			date = time.Now().UTC().Format("January 2, 2006")
		}
		line, _ := strconv.Atoi(values["__line"])
		rows = append(rows, MappedRow{Line: line, Date: date, Name: get("name"), Phone: get("phone"), Phone2: get("phone2"), Address: get("address"), Product: get("product"), Quantity: quantity, COD: get("cod_amount"), Status: get("status"), Courier: get("courier"), Location: get("location"), Remarks: get("remarks")})
	}
	return rows, nil
}

func containsHeader(headers []string, target string) bool {
	for _, header := range headers {
		if header == target {
			return true
		}
	}
	return false
}

func (i *Importer) ImportMappedProducts(ctx context.Context, table UploadedTable) ([]ReviewRow, error) {
	nameHeader := findHeader(table.Headers, "product", "product_name", "name", "item")
	if nameHeader == "" {
		return nil, fmt.Errorf("product upload needs a product name column")
	}
	priceHeader := findHeader(table.Headers, "price", "unit_price", "amount")
	existing, err := i.Queries.ListProducts(ctx, db.ListProductsParams{CompanyID: i.CompanyID, Column2: ""})
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool)
	for _, product := range existing {
		known[normalizeName(product.Name)] = true
	}
	review := make([]ReviewRow, 0)
	for _, row := range table.Rows {
		name := strings.TrimSpace(row[nameHeader])
		if name == "" || known[normalizeName(name)] {
			continue
		}
		price := pgtype.Numeric{}
		value := "0"
		if priceHeader != "" && strings.TrimSpace(row[priceHeader]) != "" {
			value = strings.ReplaceAll(strings.TrimSpace(row[priceHeader]), ",", "")
		}
		if err := price.Scan(value); err != nil {
			review = append(review, ReviewRow{Source: "products", Reason: "invalid product price", Data: row})
			continue
		}
		if _, err := i.Queries.CreateProduct(ctx, db.CreateProductParams{Name: name, Price: price, CompanyID: i.CompanyID}); err != nil {
			return review, err
		}
		known[normalizeName(name)] = true
	}
	return review, nil
}

func findHeader(headers []string, candidates ...string) string {
	for _, candidate := range candidates {
		for _, header := range headers {
			value := strings.ToLower(strings.Join(strings.Fields(header), "_"))
			if value == candidate {
				return header
			}
		}
	}
	return ""
}

func (i *Importer) ImportMappedCouriers(ctx context.Context, table UploadedTable) error {
	nameHeader := findHeader(table.Headers, "courier", "courier_name", "name", "service")
	if nameHeader == "" {
		return fmt.Errorf("courier upload needs a courier name column")
	}
	known := make(map[string]bool)
	couriers, err := i.Queries.ListCouriers(ctx, i.CompanyID)
	if err != nil {
		return err
	}
	for _, courier := range couriers {
		known[normalizeName(courier.Name)] = true
	}
	for _, row := range table.Rows {
		name := strings.TrimSpace(row[nameHeader])
		if name == "" || known[normalizeName(name)] {
			continue
		}
		if _, err := i.Queries.CreateCourier(ctx, db.CreateCourierParams{Name: name, CompanyID: i.CompanyID}); err != nil {
			return err
		}
		known[normalizeName(name)] = true
	}
	return nil
}

func (i *Importer) ImportMappedLocations(ctx context.Context, table UploadedTable) ([]ReviewRow, error) {
	locationHeader := findHeader(table.Headers, "location", "location_name", "branch", "area")
	courierHeader := findHeader(table.Headers, "courier", "courier_name", "service")
	if locationHeader == "" || courierHeader == "" {
		return nil, fmt.Errorf("location upload needs location and courier columns")
	}
	couriers, err := i.Queries.ListCouriers(ctx, i.CompanyID)
	if err != nil {
		return nil, err
	}
	courierIDs := make(map[string]pgtype.UUID)
	for _, courier := range couriers {
		courierIDs[normalizeName(courier.Name)] = courier.ID
	}
	review := make([]ReviewRow, 0)
	for _, row := range table.Rows {
		locationName := strings.TrimSpace(row[locationHeader])
		courierName := strings.TrimSpace(row[courierHeader])
		courierID, ok := courierIDs[normalizeName(courierName)]
		if locationName == "" || !ok {
			review = append(review, ReviewRow{Source: "locations", Reason: "location row has no known courier", Data: row})
			continue
		}
		locations, listErr := i.Queries.ListCourierLocations(ctx, db.ListCourierLocationsParams{CourierID: courierID, CompanyID: i.CompanyID})
		if listErr != nil {
			return review, listErr
		}
		known := false
		for _, location := range locations {
			if normalizeName(location.LocationName) == normalizeName(locationName) {
				known = true
				break
			}
		}
		if !known {
			if _, createErr := i.Queries.CreateCourierLocation(ctx, db.CreateCourierLocationParams{CourierID: courierID, CompanyID: i.CompanyID, LocationName: locationName}); createErr != nil {
				return review, createErr
			}
		}
	}
	return review, nil
}
