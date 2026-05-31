package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var direction, dbURL, migrationsPath string
	flag.StringVar(&direction, "direction", "up", "up or down")
	flag.StringVar(&dbURL, "db", "", "database URL (postgres://... or sqlite3://...)")
	flag.StringVar(&migrationsPath, "path", "", "path to migration files")
	flag.Parse()

	if dbURL == "" || migrationsPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if strings.HasPrefix(dbURL, "sqlite3://") || strings.HasPrefix(dbURL, "sqlite://") {
		dbURL = strings.Replace(dbURL, "sqlite://", "sqlite3://", 1)
	}

	m, err := migrate.New("file://"+migrationsPath, dbURL)
	if err != nil {
		log.Fatalf("migrate new: %v", err)
	}

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up: %v", err)
		}
		fmt.Println("Migration up complete")
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down: %v", err)
		}
		fmt.Println("Migration down complete")
	default:
		log.Fatalf("unknown direction: %s", direction)
	}
}
