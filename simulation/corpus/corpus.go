package corpus

import (
	"context"
	"db-seeder/config"
	"db-seeder/simulation/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jackc/pgx/v5"
)

const DefaultPath = ".walker/corpus.json"

type Entry struct {
	ID      string                `json:"id"`
	Seed    map[string]string     `json:"seed"`
	Session map[string]string     `json:"session"`
	Metrics []types.RequestMetric `json:"metrics"`
}

type Corpus struct {
	mu      sync.Mutex
	entries []Entry
	path    string
}

func New(path string) *Corpus {
	if path == "" {
		path = DefaultPath
	}
	return &Corpus{path: path}
}

func (c *Corpus) Add(e Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, e)
	return c.flush()
}

func (c *Corpus) Entries() []Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Entry, len(c.entries))
	copy(out, c.entries)
	return out
}

func (c *Corpus) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// flush writes corpus to disk — must be called with mu held.
func (c *Corpus) flush() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return fmt.Errorf("corpus: creating output dir: %w", err)
	}
	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return fmt.Errorf("corpus: marshalling: %w", err)
	}
	return os.WriteFile(c.path, data, 0644)
}
func DeleteCorpus(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("corpus: loading config: %w", err)
	}

	dbURL := cfg.Get("db_url")
	if dbURL == "" {
		return fmt.Errorf("corpus: db_url not set in config — add it via Configure")
	}

	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		return fmt.Errorf("corpus: connecting to db: %w", err)
	}
	defer conn.Close(context.Background())

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("corpus: opening file: %w", err)
	}
	defer f.Close()

	var emails []string
	dec := json.NewDecoder(f)
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("corpus: reading json: %w", err)
	}
	for dec.More() {
		var entry struct {
			Seed map[string]string `json:"seed"`
		}
		if err := dec.Decode(&entry); err != nil {
			return fmt.Errorf("corpus: decoding entry: %w", err)
		}
		if email, ok := entry.Seed["email"]; ok {
			emails = append(emails, email)
		}
	}

	if len(emails) > 0 {
		fmt.Printf("corpus: attempting to delete %d users\n", len(emails))
		for _, e := range emails {
			fmt.Printf("  - %s\n", e)
		}
		tag, err := conn.Exec(context.Background(),
			"DELETE FROM users WHERE email = ANY($1)", emails)
		if err != nil {
			return fmt.Errorf("corpus: deleting users from db: %w", err)
		}
		fmt.Printf("corpus: deleted %d rows\n", tag.RowsAffected())
	}

	return nil
}
