package main

import (
	"context"
	"log"
	"net/http"
	"oms-backend/internal/products"
	"strings"
	"time"

	"os"

	"golang.org/x/time/rate"

	"oms-backend/internal/auth"
	"oms-backend/internal/couriers"
	"oms-backend/internal/customers"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/imports"
	"oms-backend/internal/orders"
	"oms-backend/internal/reports"
	"oms-backend/internal/users"

	"github.com/go-chi/cors"

	"github.com/joho/godotenv"

	"github.com/go-chi/chi/v5"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("Warning: .env file not found")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}

	dbPool, err := connection.NewPool(databaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer dbPool.Close()

	queries := db.New(dbPool)
	if err := recoverInterruptedImports(context.Background(), queries); err != nil {
		log.Fatalf("legacy import recovery failed: %v", err)
	}

	authHandler := &auth.Handler{
		Queries: queries,
		Pool:    dbPool,
	}
	orderHandler := orders.NewHandler(queries, dbPool)
	customerHandler := customers.NewHandler(queries)
	productHandler := products.NewHandler(queries)

	courierHandler := couriers.NewHandler(queries)
	reportHandler := reports.NewHandler(queries)
	userHandler := users.NewHandler(queries)
	importHandler := imports.NewHandler(queries, dbPool)
	r := NewRouter(jwtSecret, authHandler, orderHandler, customerHandler, productHandler, courierHandler, reportHandler, userHandler, importHandler, queries)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("OMS API running on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func recoverInterruptedImports(ctx context.Context, queries *db.Queries) error {
	return queries.FailInterruptedLegacyImportRuns(ctx)
}

func NewRouter(
	jwtSecret string,
	authHandler *auth.Handler,
	orderHandler *orders.Handler,
	customerHandler *customers.Handler,
	productHandler *products.Handler,
	courierHandler *couriers.Handler,
	reportHandler *reports.Handler,
	userHandler *users.Handler,
	importHandler *imports.Handler,
	queryOptions ...*db.Queries,
) *chi.Mux {
	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	loginRateLimiter := auth.NewLoginRateLimiter(rate.Limit(5.0/60.0), 5)
	loginRateLimiter.Cleanup(10 * time.Minute)

	r.With(loginRateLimiter.Middleware).Post("/api/auth/login", authHandler.Login)
	r.Get("/api/auth/setup/status", authHandler.SetupStatus)
	r.Post("/api/auth/setup", authHandler.Setup)
	r.Get("/api/auth/invitation/{token}", authHandler.GetInvitation)
	r.Post("/api/auth/accept-invitation/{token}", authHandler.AcceptInvitation)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(jwtSecret, queryOptions...))

		r.Get("/api/me", auth.Me)

		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/customers", customerHandler.ListCustomers)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Post("/api/customers", customerHandler.CreateCustomer)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/customers/search", customerHandler.SearchByPhone)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/customers/{id}", customerHandler.GetCustomer)

		r.With(auth.RequireRole("superadmin", "admin")).Patch("/api/products/{id}", productHandler.UpdateProduct)

		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/products", productHandler.CreateProduct)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/products", productHandler.ListProducts)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/products/{id}", productHandler.GetProduct)
		r.With(auth.RequireRole("superadmin", "admin")).Put("/api/products/{id}", productHandler.UpdateProduct)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/orders/{id}", orderHandler.GetOrder)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/orders", orderHandler.ListOrders)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Post("/api/orders", orderHandler.CreateOrder)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Patch("/api/orders/{id}/status", orderHandler.UpdateStatus)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Post("/api/orders/{id}/followup", orderHandler.CreateFollowUp)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/followups", orderHandler.ListFollowUps)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/dashboard/summary", reportHandler.Summary)
		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/reports/staff-performance", reportHandler.StaffPerformance)
		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/reports/confirmed-courier-wise", reportHandler.ConfirmedCourierCounts)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/customers/{id}/history", reportHandler.CustomerHistory)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/orders/problems", reportHandler.ProblemOrders)
		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/reports/orders.csv", reportHandler.ExportOrders)

		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/couriers", courierHandler.ListCouriers)
		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/couriers", courierHandler.CreateCourier)
		r.With(auth.RequireRole("superadmin", "admin")).Patch("/api/couriers/{id}", courierHandler.UpdateCourier)
		r.With(auth.RequireRole("superadmin", "admin")).Delete("/api/couriers/{id}", courierHandler.DeleteCourier)
		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/couriers/{courierID}/locations", courierHandler.ListLocations)
		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/couriers/{courierID}/locations", courierHandler.CreateLocation)
		r.With(auth.RequireRole("superadmin", "admin")).Patch("/api/couriers/{courierID}/locations/{id}", courierHandler.UpdateLocation)
		r.With(auth.RequireRole("superadmin", "admin")).Delete("/api/couriers/{courierID}/locations/{id}", courierHandler.DeleteLocation)

		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/admin/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access":"admin"}`))
		})

		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/users", userHandler.Create)
		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/users", userHandler.List)
		r.With(auth.RequireRole("superadmin", "admin")).Patch("/api/users/{id}", userHandler.Update)
		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/users/{id}/resend-invitation", userHandler.ResendInvitation)
		r.With(auth.RequireRole("superadmin", "admin")).Post("/api/users/{id}/revoke-invitation", userHandler.RevokeInvitation)
		r.With(auth.RequireRole("superadmin")).Post("/api/imports/legacy", importHandler.Start)
		r.With(auth.RequireRole("superadmin")).Get("/api/imports/legacy", importHandler.Status)
		r.With(auth.RequireRole("superadmin")).Post("/api/imports/mapped/upload", importHandler.Upload)
		r.With(auth.RequireRole("superadmin")).Get("/api/imports/mapped/{id}/preview", importHandler.Preview)
		r.With(auth.RequireRole("superadmin")).Get("/api/imports/mapped/{id}/review", importHandler.Review)
		r.With(auth.RequireRole("superadmin")).Put("/api/imports/mapped/{id}/mapping", importHandler.SaveMapping)
		r.With(auth.RequireRole("superadmin")).Post("/api/imports/mapped/{id}/start", importHandler.SaveMappingAndStart)
		r.With(auth.RequireRole("superadmin")).Post("/api/imports/mapped/upload", importHandler.Upload)
		r.With(auth.RequireRole("superadmin")).Get("/api/imports/mapped/{id}/preview", importHandler.Preview)
		r.With(auth.RequireRole("superadmin")).Put("/api/imports/mapped/{id}/mapping", importHandler.SaveMapping)
		r.With(auth.RequireRole("superadmin")).Post("/api/imports/mapped/{id}/start", importHandler.SaveMappingAndStart)

		r.With(auth.RequireRole("staff", "admin", "superadmin")).Get("/api/staff/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access":"staff-compatible"}`))
		})
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","database":"connected"}`))
	})

	return r
}

func allowedOrigins() []string {
	value := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if value == "" {
		return []string{"http://localhost:5173"}
	}

	origins := make([]string, 0)
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return []string{"http://localhost:5173"}
	}
	return origins
}
