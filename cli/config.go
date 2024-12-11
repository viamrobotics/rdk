package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCLICachePath() string {
	return filepath.Join(viamDotDir, "cached_cli_config.json")
}

func getCLIProfilesPath() string {
	return filepath.Join(viamDotDir, "cli_profiles.json")
}

func getCLIProfilePath(profileName string) string {
	return filepath.Join(viamDotDir, fmt.Sprintf("%s_cached_cli_config.json", profileName))
}

// ConfigFromCache parses the cached json into a Config. Removes the config from cache on any error.
// TODO(RSDK-7812): maybe move shared code to common location.
func configFromCacheInner(configPath string) (_ *Config, err error) {
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

func ConfigFromCache(c *cli.Context) (*Config, error) {
	// if the global `--profile` override flag is set, use that.
	// CR erodkin: remove this `isSet` call if we can
	if c.IsSet(profileFlag) {
		// CR erodkin: use args
		conf, err := configFromCacheInner(c.String(profileFlag))
		if err != nil {
			return conf, err
		}
		// CR erodkin: use args
		warningf(c.App.ErrWriter, "Unable to find config for profile %s", c.String(profileFlag))
	}
	getProfileConfig := func() (*Config, error) {
		profiles := profiles{}
		profilesBytes, err := os.ReadFile(getCLIProfilesPath())
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal(profilesBytes, profiles); err != nil {
			return nil, err
		}

		return configFromCacheInner(getCLIProfilePath(profiles.currentProfile))
	}
	conf, err := getProfileConfig()
	if err == nil {
		return conf, nil
	}

	warningf(c.App.ErrWriter, "Unable to get config for profile, falling back to default config")
	return configFromCacheInner(getCLICachePath())
}

func removeConfigFromCache() error {
	return os.Remove(getCLICachePath())
}

func storeConfigToCacheInner(cfg *Config, path string) error {
	if err := os.MkdirAll(viamDotDir, 0o700); err != nil {
		return err
	}
	md, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	//nolint:gosec
	return os.WriteFile(path, md, 0o640)
}

func storeProfileConfigToCache(cfg *Config, profileName string) error {
	return storeConfigToCacheInner(cfg, getCLIProfilePath(profileName))
}

func storeConfigToCache(cfg *Config) error {
	return storeConfigToCacheInner(cfg, getCLICachePath())
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
