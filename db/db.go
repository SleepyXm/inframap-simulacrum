package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

var DBName = "seeder_db"

// Connect to the default postgres DB first (not your app DB)
func connectDefault() (*pgx.Conn, error) {
	godotenv.Load()
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/postgres",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
	)
	return pgx.Connect(context.Background(), url)
}

// Connect to your app DB
func Connect() (*pgx.Conn, error) {
	godotenv.Load()
	url := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		DBName,
	)
	return pgx.Connect(context.Background(), url)
}

// Create the DB if it doesn't exist
func CreateDatabase() error {
	conn, err := connectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer conn.Close(context.Background())

	// Check if DB already exists
	var exists bool
	err = conn.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", DBName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check db existence: %w", err)
	}

	if exists {
		fmt.Printf("Database %s already exists, skipping create\n", DBName)
		return nil
	}

	// CREATE DATABASE doesn't support parameterised queries so we format it directly
	// DBName is hardcoded above so this is safe
	if _, err := conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", DBName)); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	fmt.Printf("Database %s created\n", DBName)
	return nil
}

// Drop and recreate — useful for test resets
func ResetDatabase() error {
	conn, err := connectDefault()
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer conn.Close(context.Background())

	// Terminate existing connections first
	_, err = conn.Exec(context.Background(),
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", DBName,
	)

	if err != nil {
		return fmt.Errorf("failed to terminate connections: %w", err)
	}

	if _, err := conn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", DBName)); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	if _, err := conn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", DBName)); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	fmt.Printf("Database %s reset\n", DBName)
	return nil
}

// Run your schema — call this after CreateDatabase
func RunSchema(conn *pgx.Conn) error {
	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			first_name  TEXT NOT NULL,
			last_name   TEXT NOT NULL,
			email       TEXT UNIQUE NOT NULL,
			username    TEXT UNIQUE,
			password    TEXT NOT NULL,
			created_at  TIMESTAMPTZ DEFAULT NOW()
		);
	`
	if _, err := conn.Exec(context.Background(), schema); err != nil {
		return fmt.Errorf("failed to run schema: %w", err)
	}

	fmt.Println("Schema applied")
	return nil
}
