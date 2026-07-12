package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("ping: %v", err)
	}
	fmt.Println("OK Connected")
}
