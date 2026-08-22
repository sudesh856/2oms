package legacyimport

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	db "oms-backend/internal/db/generated"
)

type ImportOptions struct {
	SourceDir string
	Year      int
	Source    db.OrderSource
}

func ImportDirectory(ctx context.Context, importer *Importer, options ImportOptions) (Report, error) {
	if options.SourceDir == "" {
		return Report{}, fmt.Errorf("legacy source directory is required")
	}
	importer.Year = options.Year
	importer.Source = options.Source

	ordersFile, err := os.Open(filepath.Join(options.SourceDir, "orders.csv"))
	if err != nil {
		return Report{}, err
	}
	rows, review, err := ParseDaily(ordersFile, 0)
	ordersFile.Close()
	if err != nil {
		return Report{}, err
	}
	importer.Review = append(importer.Review, review...)
	orderCounts, err := importer.ImportDaily(ctx, rows)
	if err != nil {
		return Report{}, err
	}
	report := Report{Sources: map[string]Counts{
		"orders.csv": {Read: len(rows) + len(review), Inserted: orderCounts.Inserted, Skipped: orderCounts.Skipped + len(review)},
	}}

	dailyPhones := make(map[string]bool)
	for _, row := range rows {
		dailyPhones[NormalizePhone(row.Phone)] = true
	}
	for _, name := range []string{"ncmupaya.csv", "dsrider.csv", "pathao.csv", "doorma.csv"} {
		file, openErr := os.Open(filepath.Join(options.SourceDir, name))
		if openErr != nil {
			return Report{}, openErr
		}
		counts, handoffReview, auditErr := AuditHandOff(file, dailyPhones, name)
		file.Close()
		if auditErr != nil {
			return Report{}, auditErr
		}
		importer.Review = append(importer.Review, handoffReview...)
		report.Sources[name] = Counts{Read: counts.Read, Skipped: counts.Skipped}
	}

	productsFile, err := os.Open(filepath.Join(options.SourceDir, "products.csv"))
	if err != nil {
		return Report{}, err
	}
	productCounts, productReview, err := ImportProducts(ctx, importer.Queries, productsFile, importer.CompanyID)
	productsFile.Close()
	if err != nil {
		return Report{}, err
	}
	importer.Review = append(importer.Review, productReview...)
	report.Sources["products.csv"] = Counts{Read: productCounts.Read, Inserted: productCounts.Inserted, Skipped: productCounts.Skipped}

	locationFile, err := os.Open(filepath.Join(options.SourceDir, "location.csv"))
	if err != nil {
		return Report{}, err
	}
	locationCounts, locationReview, err := AuditLocations(ctx, importer.Queries, locationFile, importer.CompanyID)
	locationFile.Close()
	if err != nil {
		return Report{}, err
	}
	importer.Review = append(importer.Review, locationReview...)
	report.Sources["location.csv"] = Counts{Read: locationCounts.Read, Skipped: locationCounts.Skipped}

	exchangeFile, err := os.Open(filepath.Join(options.SourceDir, "exchange.csv"))
	if err != nil {
		return Report{}, err
	}
	exchangeCounts, exchangeReview, err := AuditExchange(exchangeFile)
	exchangeFile.Close()
	if err != nil {
		return Report{}, err
	}
	importer.Review = append(importer.Review, exchangeReview...)
	report.Sources["exchange.csv"] = Counts{Read: exchangeCounts.Read, Skipped: exchangeCounts.Skipped}
	report.Review = importer.Review
	return report, nil
}

func ImportRunTotals(report Report) (read, inserted, skipped int32) {
	for _, counts := range report.Sources {
		read += int32(counts.Read)
		inserted += int32(counts.Inserted)
		skipped += int32(counts.Skipped)
	}
	return read, inserted, skipped
}
