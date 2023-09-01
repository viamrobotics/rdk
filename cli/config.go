package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
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

	tokenErr := conf.tryUnmarshallWithToken(rd)
	if tokenErr == nil {
		return &conf, nil
	}
	apiKeyErr := conf.tryUnmarshallWithAPIKey(rd)
	if apiKeyErr == nil {
		return &conf, nil
	}

	return nil, errors.Wrap(multierr.Combine(tokenErr, apiKeyErr), "failed to read config from cache")
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

func (conf *config) tryUnmarshallWithToken(configBytes []byte) error {
	conf.Auth = &token{}
	if err := json.Unmarshal(configBytes, &conf); err != nil {
		return err
	}
	if conf.Auth != nil && conf.Auth.(*token).User.Email != "" {
		return nil
	}
	return errors.New("config did not contain a user token")
}

func (conf *config) tryUnmarshallWithAPIKey(configBytes []byte) error {
	conf.Auth = &apiKey{}
	if err := json.Unmarshal(configBytes, &conf); err != nil {
		return err
	}
	if conf.Auth != nil && conf.Auth.(*apiKey).KeyID != "" {
		return nil
	}
	return errors.New("config did not contain an api key")
}
