package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const DefaultPath = ".walker/config.json"

type Config map[string]string

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return nil, fmt.Errorf("config: reading file: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parsing file: %w", err)
	}
	return cfg, nil
}

func Save(cfg Config, path string) error {
	if path == "" {
		path = DefaultPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("config: creating dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshalling: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func (c Config) Get(key string) string {
	return c[key]
}

func (c Config) Set(key, value string) {
	c[key] = value
}

func (c Config) Delete(key string) {
	delete(c, key)
}

// Keys returns config keys in consistent alphabetical order for stable TUI rendering.
func (c Config) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
