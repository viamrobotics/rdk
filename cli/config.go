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

func configFromCacheInner(configPath string) (_ *Config, err error) {
	defer func() {
		if err != nil && !os.IsNotExist(err) {
			utils.UncheckedError(removeConfigFromCache())
		}
	}()
	//nolint: gosec
	rd, err := os.ReadFile(configPath)
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

// ConfigFromCache parses the cached json into a Config. Removes the config from cache on any error.
// TODO(RSDK-7812): maybe move shared code to common location.
func ConfigFromCache(c *cli.Context) (*Config, error) {
	var configPath string
	var whichProf *string
	var profileSpecified bool
	globalArgs, err := getGlobalArgs(c)
	if err != nil {
		return nil, err
	}
	if !globalArgs.DisableProfiles {
		whichProf, profileSpecified = whichProfile(globalArgs)
	}

	if whichProf != nil {
		configPath = getCLIProfilePath(*whichProf)
		conf, err := configFromCacheInner(configPath)
		if err == nil {
			conf.profile = *whichProf
			return conf, nil
		}

		// if someone explicitly asked for a profile via CLI flag and we were unable to provide
		// it, we should error out rather than trying to infer what they might prefer instead
		if profileSpecified {
			return nil, err
		}

		// A profile has been set as an env var but not specified by flag. Since the env var
		// is relatively persistent, it's more reasonable to assume a user would want to fall
		// back to default login behavior.
		warningf(c.App.ErrWriter, "Unable to find config for profile %s, falling back to default login", globalArgs.Profile)
	}

	return configFromCacheInner(getCLICachePath())
}

func removeConfigFromCache() error {
	return os.Remove(getCLICachePath())
}

func storeConfigToCache(cfg *Config) error {
	var path string

	if cfg.profile != "" {
		path = getCLIProfilePath(cfg.profile)
	} else {
		path = getCLICachePath()
	}
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

// Config is the schema for saved CLI credentials.
type Config struct {
	BaseURL         string     `json:"base_url"`
	Auth            authMethod `json:"auth"`
	LastUpdateCheck string     `json:"last_update_check"`
	LatestVersion   string     `json:"latest_version"`
	profile         string
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
