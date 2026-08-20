package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	"oms-backend/internal/auth"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
)

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("Warning: .env not found")
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

	name := os.Getenv("BOOTSTRAP_STAFF_NAME")
	phone := auth.NormalizePhone(os.Getenv("BOOTSTRAP_STAFF_PHONE"))
	password := os.Getenv("BOOTSTRAP_STAFF_PASSWORD")
	if name == "" || phone == "" || password == "" {
		log.Fatal("BOOTSTRAP_STAFF_NAME, BOOTSTRAP_STAFF_PHONE, and BOOTSTRAP_STAFF_PASSWORD must be set")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}

	queries := db.New(pool)

	user, err := queries.CreateUser(
		context.Background(),
		db.CreateUserParams{
			Name:         name,
			Phone:        phone,
			PasswordHash: hash,
			Role:         db.UserRoleStaff,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Created STAFF: %s (%s)", user.Name, user.Phone)
}
