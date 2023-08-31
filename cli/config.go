package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCLICachePath() string {
	return filepath.Join(viamDotDir, "cached_cli_config.json")
}

func configFromCache() (*config, error) {
	rd, err := os.ReadFile(getCLICachePath())
	if err != nil {
		return nil, err
	}
	var conf config

	conf.Auth = &token{}
	if err := json.Unmarshal(rd, &conf); err != nil {
		return nil, err
	}
	if conf.Auth != nil && conf.Auth.(*token).User.Email != "" {
		return &conf, nil
	}

	conf.Auth = &apiKey{}
	if err := json.Unmarshal(rd, &conf); err != nil {
		return nil, err
	}
	if conf.Auth != nil && conf.Auth.(*apiKey).KeyID != "" {
		return &conf, nil
	}

	return nil, errors.New("failed to read config from cache. auth was not an api key or a token")
}

func removeConfigFromCache() error {
	return os.Remove(getCLICachePath())
}

func storeConfigToCache(cfg *config) error {
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

type config struct {
	BaseURL string     `json:"base_url"`
	Auth    authMethod `json:"auth"`
}
