package cli

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang-jwt/jwt/v4"
	"go.viam.com/utils/rpc"
)

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCLICachePath() string {
	return filepath.Join(viamDotDir, "cached_cli_config.json")
}

func configFromCache() (*Config, error) {
	rd, err := ioutil.ReadFile(getCLICachePath())
	if err != nil {
		return nil, err
	}
	var conf Config
	if err := json.Unmarshal(rd, &conf); err != nil {
		return nil, err
	}
	if err := conf.parseAuthInfo(); err != nil {
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
	return ioutil.WriteFile(getCLICachePath(), md, 0o640)
}

// Config contains stored config information for the CLI.
type Config struct {
	Auth      string `json:"auth"` // opaque
	AuthEmail string `json:"-"`
}

func (c *Config) parseAuthInfo() error {
	var claims rpc.JWTClaims
	if _, _, err := jwt.NewParser().ParseUnverified(c.Auth, &claims); err != nil {
		return err
	}
	c.AuthEmail = claims.AuthMetadata["email"]
	return nil
}
