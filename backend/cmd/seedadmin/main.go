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

	password := "ChangeMe123!"

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatal(err)
	}

	queries := db.New(pool)

	user, err := queries.CreateUser(
		context.Background(),
		db.CreateUserParams{
			Name:         "System Administrator",
			Phone:        "9800000000",
			PasswordHash: hash,
			Role:         db.UserRoleSuperadmin,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Created SUPERADMIN: %s (%s)", user.Name, user.Phone)
	log.Println("Temporary password:", password)
}
