package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
)

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCLICachePath() string {
	return filepath.Join(viamDotDir, "cached_cli_config.json")
}

func configFromCache() (*Config, error) {
	rd, err := os.ReadFile(getCLICachePath())
	if err != nil {
		return nil, err
	}
	var conf Config
	if err := json.Unmarshal(rd, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func removeConfigFromCache() error {
	return os.Remove(getCLICachePath())
}

func storeConfigToCache(cfg *Config) error {
	if err := os.MkdirAll(viamDotDir, 0o700); err != nil {
		return err
	}
	md, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	return os.WriteFile(getCLICachePath(), md, 0o640)
}

// Config contains stored config information for the CLI.
type Config struct {
	Auth *Token `json:"auth"`
}
