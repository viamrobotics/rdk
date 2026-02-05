package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

// CLIConfig is a simpler version of cli's config: it holds the minimum data needed to connect to app
type CLIConfig struct {
	BaseURL string `json:"base_url"`
	Auth    struct {
		AccessToken string `json:"access_token"`
		KeyID       string `json:"key_id"`
		KeyCrypto   string `json:"key_crypto"`
	} `json:"auth"`
}

// GetCLICachePath returns the path to the cached CLI config.
func GetCLICachePath() string {
	return filepath.Join(ViamDotDir, "cached_cli_config.json")
}

// ParseBaseURL parses a base URL and returns the parsed URL and any necessary dial options.
// If verifyConnection is true, it will attempt a TCP connection to verify the URL is reachable.
func ParseBaseURL(baseURL string, verifyConnection bool) (*url.URL, []rpc.DialOption, error) {
	baseURLParsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, nil, err
	}

	// Go URL parsing can place the host in Path if no scheme is provided; place
	// Path in Host in this case.
	if baseURLParsed.Host == "" && baseURLParsed.Path != "" {
		baseURLParsed.Host = baseURLParsed.Path
		baseURLParsed.Path = ""
	}

	// Assume "https" scheme if none is provided, and assume 8080 port for "http"
	// scheme and 443 port for "https" scheme.
	var secure bool
	switch baseURLParsed.Scheme {
	case "http":
		if baseURLParsed.Port() == "" {
			baseURLParsed.Host = baseURLParsed.Host + ":" + "8080"
		}
	case "https", "":
		secure = true
		baseURLParsed.Scheme = "https"
		if baseURLParsed.Port() == "" {
			baseURLParsed.Host = baseURLParsed.Host + ":" + "443"
		}
	}

	if verifyConnection {
		// Check if URL is even valid with a TCP dial.
		conn, err := net.DialTimeout("tcp", baseURLParsed.Host, 10*time.Second)
		if err != nil {
			return nil, nil, fmt.Errorf("base URL %q (needed for auth) is currently unreachable (%v). "+
				"Ensure URL is valid and you are connected to internet", err.Error(), baseURLParsed.Host)
		}
		utils.UncheckedError(conn.Close())
	}

	if secure {
		return baseURLParsed, nil, nil
	}
	return baseURLParsed, []rpc.DialOption{
		rpc.WithInsecure(),
		rpc.WithAllowInsecureWithCredentialsDowngrade(),
	}, nil
}

// DialOptions constructs dial options from the config data.
func (c *CLIConfig) DialOptions() ([]rpc.DialOption, error) {
	_, baseOpts, err := ParseBaseURL(c.BaseURL, true)
	if err != nil {
		return nil, err
	}

	var authOpt rpc.DialOption
	if c.Auth.AccessToken != "" {
		authOpt = rpc.WithStaticAuthenticationMaterial(c.Auth.AccessToken)
	} else if c.Auth.KeyID != "" {
		authOpt = rpc.WithEntityCredentials(c.Auth.KeyID, rpc.Credentials{
			Type:    "api-key",
			Payload: c.Auth.KeyCrypto,
		})
	} else {
		return nil, fmt.Errorf("config does not contain valid token or API key")
	}

	return append(baseOpts, authOpt), nil
}

// ConfigFromPath reads CLI credentials from the specified config file path and returns a CLIConfig.
func ConfigFromPath(configPath string) (*CLIConfig, error) {
	//nolint:gosec
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no cached CLI credentials found at %q, run 'viam login' first", configPath)
		}
		return nil, fmt.Errorf("failed to read CLI config: %w", err)
	}

	var configData CLIConfig
	if err := json.Unmarshal(data, &configData); err != nil {
		return nil, fmt.Errorf("failed to parse CLI config: %w", err)
	}

	return &configData, nil
}
