// Package app contains all logic needed for communication and interaction with app.
package app

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
)

// ViamClient is a gRPC client for method calls to Viam app.
type ViamClient struct {
	conn rpc.ClientConn
}

var dialDirectGRPC = rpc.DialDirectGRPC

// CreateViamClient creates a ViamClient with an API Key.
func CreateViamClient(ctx context.Context, baseURL, apiKey, apiKeyID string, logger logging.Logger) (*ViamClient, error) {
	if baseURL == "" {
		baseURL = "https://app.viam.com"
	} else if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, errors.New("use valid serviceHost URL")
	}
	serviceHost, err := url.Parse(baseURL + ":443")
	if err != nil {
		return nil, err
	}

	switch {
	case apiKey == "" || apiKeyID == "":
		return nil, errors.New("API key and API key ID cannot be empty")
	case !validateAPIKeyFormat(apiKey):
		return nil, errors.New("API key should be a 32-char all-lowercase alphanumeric string")
	case !validateAPIKeyIDFormat(apiKeyID):
		return nil, errors.New("API key ID should be an all-lowercase alphanumeric string with this format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")
	}
	opts := rpc.WithEntityCredentials(
		apiKeyID,
		rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey,
		},
	)

	conn, err := dialDirectGRPC(ctx, serviceHost.Host, logger, opts)
	if err != nil {
		return nil, err
	}
	return &ViamClient{conn: conn}, nil
}

func (c *ViamClient) Close() error {
	return c.conn.Close()
}

func validateAPIKeyFormat(apiKey string) bool {
	regex := regexp.MustCompile("^[a-z0-9]{32}$")
	return regex.MatchString(apiKey)
}

func validateAPIKeyIDFormat(apiKeyID string) bool {
	regex := regexp.MustCompile("^[a-z0-9]{8}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{12}")
	return regex.MatchString(apiKeyID)
}
