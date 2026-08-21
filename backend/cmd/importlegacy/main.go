package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"

	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/legacyimport"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		_ = godotenv.Load(".env")
	}
	sourceDir := flag.String("source-dir", "", "directory containing source CSV files")
	sourceURL := flag.String("source-url", legacyimport.ConfiguredSourceURL(), "authoritative CSV base URL")
	reviewPath := flag.String("review-log", "legacy-import-review.jsonl", "JSONL manual-review log")
	year := flag.Int("year", 0, "year for tab dates, required")
	source := flag.String("source", "", "OMS order source enum, required")
	createdBy := flag.String("created-by", "", "UUID of the importing user, required")
	sample := flag.Int("sample-rows", 0, "import at most this many valid daily rows")
	flag.Parse()
	if *year == 0 || *source == "" || *createdBy == "" {
		flag.Usage()
		log.Fatal("year, source, and created-by are required")
	}
	creator, err := uuid.Parse(*createdBy)
	if err != nil {
		log.Fatal("invalid created-by UUID")
	}
	if *sourceDir == "" {
		if strings.TrimSpace(*sourceURL) == "" {
			log.Fatal("--source-url or LEGACY_SOURCE_URL is required when --source-dir is not provided")
		}
		*sourceDir = "legacy-source"
		if err := legacyimport.DownloadSources(*sourceDir, *sourceURL); err != nil {
			log.Fatal(err)
		}
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	creatorContext, err := db.New(pool).GetUserAuthContext(context.Background(), pgtypeUUID(creator))
	if err != nil || !creatorContext.IsActive {
		log.Fatal("created-by user does not exist or is inactive")
	}
	companyID := creatorContext.CompanyID

	file, err := os.Open(*sourceDir + string(os.PathSeparator) + "orders.csv")
	if err != nil {
		log.Fatal(err)
	}
	rows, review, err := legacyimport.ParseDaily(file, *sample)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	importer := &legacyimport.Importer{Pool: pool, Queries: db.New(pool), CreatedBy: pgtypeUUID(creator), CompanyID: companyID, Source: db.OrderSource(*source), Year: *year, Review: review}
	counts, err := importer.ImportDaily(context.Background(), rows)
	if err != nil {
		log.Fatal(err)
	}
	dailyPhones := make(map[string]bool)
	for _, row := range rows {
		dailyPhones[legacyimport.NormalizePhone(row.Phone)] = true
	}
	for _, name := range []string{"ncmupaya.csv", "dsrider.csv", "pathao.csv", "doorma.csv"} {
		handoffFile, openErr := os.Open(*sourceDir + string(os.PathSeparator) + name)
		if openErr != nil {
			log.Fatal(openErr)
		}
		handoffCounts, handoffReview, auditErr := legacyimport.AuditHandOff(handoffFile, dailyPhones, name)
		handoffFile.Close()
		if auditErr != nil {
			log.Fatal(auditErr)
		}
		importer.Review = append(importer.Review, handoffReview...)
		log.Printf("%s read=%d matched_daily=%d skipped=%d", name, handoffCounts.Read, handoffCounts.Matched, handoffCounts.Skipped)
	}

	productsFile, err := os.Open(*sourceDir + string(os.PathSeparator) + "products.csv")
	if err != nil {
		log.Fatal(err)
	}
	productCounts, productReview, err := legacyimport.ImportProducts(context.Background(), db.New(pool), productsFile, companyID)
	productsFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	importer.Review = append(importer.Review, productReview...)
	log.Printf("products.csv read=%d inserted=%d matched=%d skipped=%d", productCounts.Read, productCounts.Inserted, productCounts.Matched, productCounts.Skipped)

	locationFile, err := os.Open(*sourceDir + string(os.PathSeparator) + "location.csv")
	if err != nil {
		log.Fatal(err)
	}
	locationCounts, locationReview, err := legacyimport.AuditLocations(context.Background(), db.New(pool), locationFile, companyID)
	locationFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	importer.Review = append(importer.Review, locationReview...)
	log.Printf("location.csv read=%d matched_seed=%d skipped=%d", locationCounts.Read, locationCounts.Matched, locationCounts.Skipped)

	exchangeFile, err := os.Open(*sourceDir + string(os.PathSeparator) + "exchange.csv")
	if err != nil {
		log.Fatal(err)
	}
	exchangeCounts, exchangeReview, err := legacyimport.AuditExchange(exchangeFile)
	exchangeFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	importer.Review = append(importer.Review, exchangeReview...)
	log.Printf("exchange.csv read=%d matched=%d skipped=%d", exchangeCounts.Read, exchangeCounts.Matched, exchangeCounts.Skipped)

	if err := writeReview(*reviewPath, importer.Review); err != nil {
		log.Fatal(err)
	}
	log.Printf("orders.csv read=%d inserted=%d skipped=%d review_log=%s", counts.Read, counts.Inserted, counts.Skipped+len(review), *reviewPath)
	log.Println("Hand-off rows were audited for daily-order duplicates; no duplicate hand-off orders were inserted.")
}

func pgtypeUUID(value uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: value, Valid: true} }

func writeReview(path string, rows []legacyimport.ReviewRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
