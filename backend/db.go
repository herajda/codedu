package main

import (
	"log"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sqlx.DB

// schemaSQL holds the contents of schema.sql once loaded
var schemaSQL string

func InitDB() {
	_ = godotenv.Load()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	schemaBytes, err := os.ReadFile("schema.sql")
	if err != nil {
		log.Fatalf("could not read schema.sql: %v", err)
	}
	schemaSQL = string(schemaBytes)
	var db *sqlx.DB
	for i := 1; i <= 10; i++ {
		db, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			break
		}
		log.Printf("DB connect attempt %d failed: %v", i, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("DB connect error: %v", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		log.Fatalf("schema init failed: %v", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	DB = db
	log.Println("✅ Connected to Postgres")
}
