package cli

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCLICachePath() string {
	return filepath.Join(viamDotDir, "cached_cli_config.json")
}

// ConfigFromCache parses the cached json into a Config. Removes the config from cache on any error.
// TODO(RSDK-7812): maybe move shared code to common location.
func ConfigFromCache() (_ *Config, err error) {
	defer func() {
		if err != nil && !os.IsNotExist(err) {
			utils.UncheckedError(removeConfigFromCache())
		}
	}()
	rd, err := os.ReadFile(getCLICachePath())
	if err != nil {
		return nil, err
	}
	var conf Config

	tokenErr := conf.tryUnmarshallWithToken(rd)
	if tokenErr == nil {
		return &conf, nil
	}
	apiKeyErr := conf.tryUnmarshallWithAPIKey(rd)
	if apiKeyErr == nil {
		return &conf, nil
	}

	return nil, errors.Wrap(multierr.Combine(tokenErr, apiKeyErr), "failed to parse cached config")
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

// Config is the schema for saved CLI credentials.
type Config struct {
	BaseURL         string     `json:"base_url"`
	Auth            authMethod `json:"auth"`
	LastUpdateCheck string     `json:"last_update_check"`
	LatestVersion   string     `json:"latest_version"`
}

func (conf *Config) tryUnmarshallWithToken(configBytes []byte) error {
	conf.Auth = &token{}
	if err := json.Unmarshal(configBytes, &conf); err != nil {
		return err
	}
	if conf.Auth != nil && conf.Auth.(*token).User.Email != "" {
		return nil
	}
	return errors.New("config did not contain a user token")
}

func (conf *Config) tryUnmarshallWithAPIKey(configBytes []byte) error {
	conf.Auth = &apiKey{}
	if err := json.Unmarshal(configBytes, &conf); err != nil {
		return err
	}
	if conf.Auth != nil && conf.Auth.(*apiKey).KeyID != "" {
		return nil
	}
	return errors.New("config did not contain an api key")
}

// DialOptions constructs an rpc.DialOption slice from config.
func (conf *Config) DialOptions() ([]rpc.DialOption, error) {
	_, opts, err := parseBaseURL(conf.BaseURL, true)
	if err != nil {
		return nil, err
	}
	return append(opts, conf.Auth.dialOpts()), nil
}
