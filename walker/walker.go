package walker

import (
	"db-seeder/walker/types"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func WriteJSON(v any, cfg types.JSONOutputConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	var (
		data []byte
		err  error
	)

	if cfg.Pretty {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}

	if err != nil {
		return fmt.Errorf("marshalling JSON: %w", err)
	}

	return os.WriteFile(cfg.Path, data, 0644)
}

func WriteYAML(v any, cfg types.YAMLOutputConfig) error {
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling YAML: %w", err)
	}

	return os.WriteFile(cfg.Path, data, 0644)
}
