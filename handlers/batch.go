package handlers

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func InsertBatch(conn *pgx.Conn, batchSize int, totalRecords int) error {
	people := SeedUser(totalRecords)
	query := "INSERT INTO users (first_name, last_name, email, password, created_at) VALUES ($1, $2, $3, $4, NOW())"

	// Loop through people in chunks of batchSize
	for i := 0; i < len(people); i += batchSize {
		end := i + batchSize
		if end > len(people) {
			end = len(people) // clamp the last batch
		}
		chunk := people[i:end]

		// Use pgx Batch for efficient multi-row inserts
		batch := &pgx.Batch{}
		for _, p := range chunk {
			batch.Queue(query, p.Firstname, p.Lastname, p.Email, p.Password)
		}

		results := conn.SendBatch(context.Background(), batch)

		// Execute and check each queued insert
		for range chunk {
			_, err := results.Exec()
			if err != nil {
				results.Close()
				return fmt.Errorf("batch insert failed: %w", err)
			}
		}

		if err := results.Close(); err != nil {
			return fmt.Errorf("closing batch failed: %w", err)
		}

		fmt.Println("Seeding complete!")
		fmt.Printf("Inserted records %d–%d\n", i+1, end)
	}

	return nil
}

func RecursiveRemove(conn *pgx.Conn) error {
	// This function will remove records inside of the database
	query := "DELETE FROM users"
	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return fmt.Errorf("failed to remove records: %w", err)
	}
	return nil
}

func RemoveAll(conn *pgx.Conn) error {
	_, err := conn.Exec(context.Background(), "DELETE FROM users")
	return err
}

func RemoveByField(conn *pgx.Conn, value string) error {
	// adjust the WHERE clause to match whatever field makes sense
	_, err := conn.Exec(context.Background(), "DELETE FROM users WHERE email = $1", value)
	return err
}
