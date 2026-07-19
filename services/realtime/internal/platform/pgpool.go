package platform

import (
	"database/sql"
	"os"
	"strconv"
	"time"
)

// ConfigurePostgresPool is an exported wrapper for configurePostgresPool,
// used by main.go to configure the shared connection pool.
func ConfigurePostgresPool(db *sql.DB, maxOpenDefault, maxIdleDefault int) {
	configurePostgresPool(db, maxOpenDefault, maxIdleDefault)
}

func configurePostgresPool(db *sql.DB, maxOpenDefault, maxIdleDefault int) {
	maxOpen := envIntOrDefault("PG_MAX_OPEN_CONNS", maxOpenDefault)
	maxIdle := envIntOrDefault("PG_MAX_IDLE_CONNS", maxIdleDefault)
	lifetime := envDurationOrDefault("PG_CONN_MAX_LIFETIME", 5*time.Minute)
	idleTime := envDurationOrDefault("PG_CONN_MAX_IDLE_TIME", 3*time.Minute)
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(idleTime)
}

func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func envDurationOrDefault(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultVal
}
