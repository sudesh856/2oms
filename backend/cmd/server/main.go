package main

import (
	"log"
	"net/http"
	"oms-backend/internal/products"
	"time"

	"os"

	"golang.org/x/time/rate"

	"oms-backend/internal/auth"
	"oms-backend/internal/customers"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/orders"

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

	authHandler := &auth.Handler{
		Queries: queries,
	}
	orderHandler := orders.NewHandler(queries, dbPool)
	customerHandler := customers.NewHandler(queries)
	productHandler := products.NewHandler(queries)

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	loginRateLimiter := auth.NewLoginRateLimiter(rate.Limit(5.0/60.0), 5)
	loginRateLimiter.Cleanup(10 * time.Minute)

	r.With(loginRateLimiter.Middleware).Post("/api/auth/login", authHandler.Login)

	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAuth(jwtSecret))

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
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/orders/{id}/history", orderHandler.GetOrderHistory)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Get("/api/orders", orderHandler.ListOrders)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Post("/api/orders", orderHandler.CreateOrder)
		r.With(auth.RequireRole("superadmin", "admin", "staff")).Patch("/api/orders/{id}/status", orderHandler.UpdateStatus)

		r.With(auth.RequireRole("superadmin", "admin")).Get("/api/admin/test", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access":"admin"}`))
		})

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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("OMS API running on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
