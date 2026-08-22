package legacyimport

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	db "oms-backend/internal/db/generated"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var nonDigits = regexp.MustCompile(`\D`)

func ConfiguredSourceURL() string {
	return strings.TrimSpace(os.Getenv("LEGACY_SOURCE_URL"))
}

type DailyRow struct {
	Line     int
	Tab      string
	Name     string
	Phone    string
	Phone2   string
	Address  string
	Product  string
	COD      string
	Status   string
	Remarks  string
	GBL      string
	NCM      string
	Delivery string
}

type ReviewRow struct {
	Source string `json:"source"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	Data   any    `json:"data,omitempty"`
}

type Counts struct {
	Read     int `json:"read"`
	Inserted int `json:"inserted"`
	Skipped  int `json:"skipped"`
}

type Report struct {
	Sources map[string]Counts `json:"sources"`
	Review  []ReviewRow       `json:"review"`
}

type ReferenceCounts struct {
	Read     int
	Inserted int
	Skipped  int
	Matched  int
}

func NormalizePhone(value string) string {
	digits := nonDigits.ReplaceAllString(value, "")
	if len(digits) < 10 {
		return ""
	}
	return digits[len(digits)-10:]
}

func LegacySourceKey(row DailyRow) string {
	value := strings.Join([]string{
		normalizeName(row.Tab), NormalizePhone(row.Phone), normalizeName(row.Name),
		normalizeName(row.Address), normalizeName(row.Product), strings.TrimSpace(row.COD),
	}, "\x1f")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
}

func ParseDaily(reader io.Reader, limit int) ([]DailyRow, []ReviewRow, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if err != nil {
		return nil, nil, err
	}
	indexes := make(map[string]int)
	for index, value := range header {
		indexes[strings.ToLower(strings.TrimSpace(value))] = index
	}
	required := []string{"tab", "name", "address", "phone", "product", "cod"}
	for _, key := range required {
		if _, ok := indexes[key]; !ok {
			return nil, nil, fmt.Errorf("daily CSV missing %q column", key)
		}
	}

	rows := make([]DailyRow, 0)
	review := make([]ReviewRow, 0)
	for {
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, nil, readErr
		}
		line := csvReader.InputOffset()
		get := func(key string) string {
			index := indexes[key]
			if index >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[index])
		}
		row := DailyRow{
			Line: int(line), Tab: get("tab"), Name: get("name"), Phone: get("phone"),
			Phone2: get("phone2"), Address: get("address"), Product: get("product"),
			COD: get("cod"), Status: get("status"), Remarks: get("remarks"),
			GBL: get("gbl"), NCM: get("ncm"), Delivery: get("delivery"),
		}
		if row.Name == "" || NormalizePhone(row.Phone) == "" || row.Address == "" || row.Product == "" || row.COD == "" {
			review = append(review, ReviewRow{Source: "orders.csv", Line: int(line), Reason: "missing required customer, address, product, or COD field", Data: row})
			continue
		}
		rows = append(rows, row)
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, review, nil
}

func ParseDate(value string, year int) (time.Time, error) {
	value = strings.TrimSpace(value)
	layouts := []string{"January 2, 2006", "January 2", "Jan 2, 2006", "Jan 2", "Jan2", "January2"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			if parsed.Year() == 0 || !strings.Contains(layout, "2006") {
				parsed = parsed.AddDate(year-parsed.Year(), 0, 0)
			}
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable tab date %q", value)
}

func NormalizeStatus(status, remarks string) (db.OrderStatus, string) {
	value := strings.ToLower(strings.TrimSpace(status))
	switch value {
	case "cancelled":
		return db.OrderStatusCancelled, ""
	case "confirmed", "store visit", "":
		return db.OrderStatusConfirmed, ""
	case "pnr":
		return db.OrderStatusConfirmed, ""
	case "follow up", "will call self", "switched off", "busy":
		return db.OrderStatusFollowUp, "no_answer"
	default:
		if strings.Contains(strings.ToLower(remarks), "off") || strings.Contains(strings.ToLower(remarks), "samparka") {
			return db.OrderStatusFollowUp, "no_answer"
		}
		return db.OrderStatusConfirmed, ""
	}
}

func DownloadSources(dir, baseURL string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, name := range []string{"orders.csv", "products.csv", "location.csv", "ncmupaya.csv", "dsrider.csv", "pathao.csv", "doorma.csv", "exchange.csv"} {
		response, err := http.Get(strings.TrimRight(baseURL, "/") + "/" + name)
		if err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return fmt.Errorf("download %s returned %s", name, response.Status)
		}
		file, err := os.Create(dir + string(os.PathSeparator) + name)
		if err != nil {
			response.Body.Close()
			return err
		}
		_, copyErr := io.Copy(file, response.Body)
		file.Close()
		response.Body.Close()
		if copyErr != nil {
			return fmt.Errorf("save %s: %w", name, copyErr)
		}
	}
	return nil
}

func AuditHandOff(reader io.Reader, dailyPhones map[string]bool, source string) (ReferenceCounts, []ReviewRow, error) {
	rows, err := readRecords(reader)
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	counts := ReferenceCounts{Read: len(rows)}
	review := make([]ReviewRow, 0)
	for line, row := range rows {
		phones := make([]string, 0)
		meaningful := false
		for key, value := range row {
			if strings.Contains(strings.ToUpper(key), "PHONE") {
				if normalized := NormalizePhone(value); normalized != "" {
					phones = append(phones, normalized)
				}
			}
			if strings.TrimSpace(value) != "" {
				meaningful = true
			}
		}
		if !meaningful {
			continue
		}
		if len(phones) == 0 {
			review = append(review, ReviewRow{Source: source, Line: line, Reason: "meaningful hand-off row has no usable phone", Data: row})
			counts.Skipped++
			continue
		}
		matched := false
		for _, phone := range phones {
			if dailyPhones[phone] {
				matched = true
				break
			}
		}
		if matched {
			counts.Matched++
		} else {
			review = append(review, ReviewRow{Source: source, Line: line, Reason: "unmatched hand-off row requires manual review", Data: row})
		}
	}
	return counts, review, nil
}

func ImportProducts(ctx context.Context, queries *db.Queries, reader io.Reader, companyIDs ...pgtype.UUID) (ReferenceCounts, []ReviewRow, error) {
	companyID := pgtype.UUID{}
	if len(companyIDs) > 0 {
		companyID = companyIDs[0]
	}
	rows, err := readRecords(reader)
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	existing, err := queries.ListProducts(ctx, db.ListProductsParams{CompanyID: companyID, Column2: ""})
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	known := make(map[string]bool)
	for _, product := range existing {
		known[normalizeName(product.Name)] = true
	}
	counts := ReferenceCounts{Read: len(rows)}
	review := make([]ReviewRow, 0)
	for line, row := range rows {
		name := strings.TrimSpace(row["Product"])
		if name == "" {
			counts.Skipped++
			review = append(review, ReviewRow{Source: "products.csv", Line: line, Reason: "missing product name", Data: row})
			continue
		}
		key := normalizeName(name)
		if known[key] {
			counts.Matched++
			continue
		}
		priceValue := strings.TrimSpace(row["Price"])
		price := pgtype.Numeric{}
		if priceValue == "" {
			counts.Skipped++
			review = append(review, ReviewRow{Source: "products.csv", Line: line, Reason: "missing product price", Data: row})
			continue
		}
		if err := price.Scan(strings.ReplaceAll(priceValue, ",", "")); err != nil {
			counts.Skipped++
			review = append(review, ReviewRow{Source: "products.csv", Line: line, Reason: "invalid product price", Data: row})
			continue
		}
		available := numericInt(row["Final Stock"])
		warehouse := numericInt(row["Godam"])
		if _, err := queries.CreateProduct(ctx, db.CreateProductParams{Name: name, Price: price, AvailableQty: available, WarehouseQty: warehouse, CompanyID: companyID}); err != nil {
			return counts, review, err
		}
		known[key] = true
		counts.Inserted++
	}
	return counts, review, nil
}

func AuditExchange(reader io.Reader) (ReferenceCounts, []ReviewRow, error) {
	rows, err := readRecords(reader)
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	counts := ReferenceCounts{Read: len(rows)}
	review := make([]ReviewRow, 0)
	for line, row := range rows {
		meaningful := false
		for _, value := range row {
			if strings.TrimSpace(value) != "" {
				meaningful = true
				break
			}
		}
		if !meaningful {
			continue
		}
		phone := NormalizePhone(row["Phone 1"])
		if phone == "" || strings.TrimSpace(row["order id"]) == "" {
			counts.Skipped++
			review = append(review, ReviewRow{Source: "exchange.csv", Line: line, Reason: "exchange row lacks a usable phone or order id; no exchange table exists", Data: row})
			continue
		}
		counts.Matched++
	}
	return counts, review, nil
}

func AuditLocations(ctx context.Context, queries *db.Queries, reader io.Reader, companyIDs ...pgtype.UUID) (ReferenceCounts, []ReviewRow, error) {
	companyID := pgtype.UUID{}
	if len(companyIDs) > 0 {
		companyID = companyIDs[0]
	}
	rows, err := readRecords(reader)
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	couriers, err := queries.ListCouriers(ctx, companyID)
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	var ncm pgtype.UUID
	for _, courier := range couriers {
		if strings.EqualFold(courier.Name, "NCM") {
			ncm = courier.ID
		}
	}
	if !ncm.Valid {
		return ReferenceCounts{}, nil, fmt.Errorf("NCM courier is not seeded")
	}
	locations, err := queries.ListCourierLocations(ctx, db.ListCourierLocationsParams{CourierID: ncm, CompanyID: companyID})
	if err != nil {
		return ReferenceCounts{}, nil, err
	}
	known := make(map[string]bool)
	for _, location := range locations {
		known[normalizeName(location.LocationName)] = true
	}
	counts := ReferenceCounts{Read: len(rows)}
	review := make([]ReviewRow, 0)
	for line, row := range rows {
		name := strings.TrimSpace(row["NCM Locations"])
		if name == "" {
			counts.Skipped++
			continue
		}
		if known[normalizeName(name)] {
			counts.Matched++
		} else {
			counts.Skipped++
			review = append(review, ReviewRow{Source: "location.csv", Line: line, Reason: "legacy NCM location not seeded", Data: row})
		}
	}
	return counts, review, nil
}

func readRecords(reader io.Reader) ([]map[string]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]int)
	keys := make([]string, len(header))
	for index, value := range header {
		key := strings.TrimSpace(value)
		seen[key]++
		if seen[key] > 1 {
			key = fmt.Sprintf("%s_%d", key, seen[key])
		}
		keys[index] = key
	}
	rows := make([]map[string]string, 0)
	for {
		values, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		row := make(map[string]string)
		for index, key := range keys {
			if index < len(values) {
				row[key] = strings.TrimSpace(values[index])
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func numericInt(value string) int32 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return int32(parsed)
}

type Importer struct {
	Pool      *pgxpool.Pool
	Queries   *db.Queries
	CreatedBy pgtype.UUID
	CompanyID pgtype.UUID
	Source    db.OrderSource
	Year      int
	Review    []ReviewRow
}

func (i *Importer) ImportDaily(ctx context.Context, rows []DailyRow) (Counts, error) {
	counts := Counts{Read: len(rows)}
	products, err := i.Queries.ListProducts(ctx, db.ListProductsParams{CompanyID: i.CompanyID, Column2: ""})
	if err != nil {
		return counts, err
	}
	productByName := make(map[string]db.ListProductsRow)
	for _, product := range products {
		productByName[normalizeName(product.Name)] = product
	}
	for _, row := range rows {
		product, ok := productByName[normalizeName(row.Product)]
		if !ok {
			i.Review = append(i.Review, ReviewRow{Source: "orders.csv", Line: row.Line, Reason: "product not found", Data: row})
			counts.Skipped++
			continue
		}
		createdAt, err := ParseDate(row.Tab, i.Year)
		if err != nil {
			i.Review = append(i.Review, ReviewRow{Source: "orders.csv", Line: row.Line, Reason: err.Error(), Data: row})
			counts.Skipped++
			continue
		}
		status, followUpAction := NormalizeStatus(row.Status, row.Remarks)
		cod := pgtype.Numeric{}
		if err := cod.Scan(strings.ReplaceAll(row.COD, ",", "")); err != nil {
			i.Review = append(i.Review, ReviewRow{Source: "orders.csv", Line: row.Line, Reason: "invalid COD", Data: row})
			counts.Skipped++
			continue
		}
		customerID, err := i.findOrCreateCustomer(ctx, row)
		if err != nil {
			return counts, err
		}
		tx, err := i.Pool.Begin(ctx)
		if err != nil {
			return counts, err
		}
		queries := db.New(tx)
		order, err := queries.CreateLegacyOrder(ctx, db.CreateLegacyOrderParams{
			CustomerID: customerID, Source: i.Source, Status: status, Address: row.Address,
			CodAmount: cod, IsStoreVisit: strings.EqualFold(row.Status, "store visit"),
			CreatedBy: i.CreatedBy, CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
			CompanyID: i.CompanyID, LegacySourceKey: pgtype.Text{String: LegacySourceKey(row), Valid: true},
		})
		if err == pgx.ErrNoRows {
			_ = tx.Rollback(ctx)
			counts.Skipped++
			continue
		}
		if err == nil {
			_, err = queries.CreateOrderItem(ctx, db.CreateOrderItemParams{OrderID: order.ID, ProductID: product.ID, Quantity: 1, Price: product.Price, CompanyID: i.CompanyID})
		}
		if err == nil {
			_, err = queries.CreateStatusHistory(ctx, db.CreateStatusHistoryParams{OrderID: order.ID, ToStatus: status, ChangedBy: i.CreatedBy, CompanyID: i.CompanyID})
		}
		if err == nil && followUpAction != "" {
			_, err = queries.CreateFollowUp(ctx, db.CreateFollowUpParams{OrderID: order.ID, AttemptNo: 1, NextAction: pgtype.Text{String: followUpAction, Valid: true}, Note: pgtype.Text{String: row.Remarks, Valid: row.Remarks != ""}, AssignedTo: i.CreatedBy, CompanyID: i.CompanyID})
		}
		if err != nil {
			tx.Rollback(ctx)
			return counts, err
		}
		if err := tx.Commit(ctx); err != nil {
			return counts, err
		}
		counts.Inserted++
	}
	return counts, nil
}

func (i *Importer) findOrCreateCustomer(ctx context.Context, row DailyRow) (pgtype.UUID, error) {
	phone := NormalizePhone(row.Phone)
	customer, err := i.Queries.GetCustomerByPhone(ctx, db.GetCustomerByPhoneParams{Phone: phone, CompanyID: i.CompanyID})
	if err == nil {
		return customer.ID, nil
	}
	if err != pgx.ErrNoRows {
		return pgtype.UUID{}, err
	}
	customer, err = i.Queries.CreateCustomer(ctx, db.CreateCustomerParams{Phone: phone, Phone2: pgtype.Text{String: NormalizePhone(row.Phone2), Valid: NormalizePhone(row.Phone2) != ""}, Name: row.Name, Address: pgtype.Text{String: row.Address, Valid: true}, CompanyID: i.CompanyID})
	if err != nil {
		return pgtype.UUID{}, err
	}
	return customer.ID, nil
}

func normalizeName(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func ParseInt(value string) (int32, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	return int32(parsed), err
}
