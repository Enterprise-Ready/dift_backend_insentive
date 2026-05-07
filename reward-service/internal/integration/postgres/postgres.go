package postgres

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"

	"reward-service/config"
)

func NewPostgres(cfg config.DBConfig) (*sql.DB, error) {

	db, err := sql.Open("postgres", cfg.DSN)
	if err != nil {
		return nil, err
	}

	// connection pool config
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// ping test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}

	log.Println("[DB] PostgreSQL connected")
	return db, nil
}

func Close(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}
