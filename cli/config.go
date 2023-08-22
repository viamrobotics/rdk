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
	if conf.Auth.prettyPrint() != "" {
		return &conf, nil
	}

	conf.Auth = &apiKey{}
	if err := json.Unmarshal(rd, &conf); err != nil {
		return nil, err
	}


	return &conf, nil
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
	Auth authMethod `json:"auth"`
}
