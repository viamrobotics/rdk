package app

import (
	"context"
	"fmt"
	"net/url"
	"regexp"

	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

type ViamClient struct {
	conn rpc.ClientConn
}

func CreateViamClient(ctx context.Context, baseURL string, apiKey string, apiKeyID string, logger logging.Logger) (*ViamClient, error) {
	if baseURL == "" {
		baseURL = "https://app.viam.com"
	}
	serviceHost, err := url.Parse(baseURL + ":443")
	if err != nil {
		return nil, err
	}

	switch {
	case apiKey == "" || apiKeyID == "":
		return nil, fmt.Errorf("API key and API key ID cannot be empty.")
	case !validateApiKeyFormat(apiKey):
		return nil, fmt.Errorf("API key should be a 32-char all-lowercase alphanumeric string")
	case !validateApiKeyIDFormat(apiKeyID):
		return nil, fmt.Errorf("API key ID should be an all-lowercase alphanumeric string with this format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx")
	}
	opts := rpc.WithEntityCredentials(
		apiKeyID,
		rpc.Credentials{
			Type: rpc.CredentialsTypeAPIKey,
			Payload: apiKey,
		},
	)

	conn, err := rpc.DialDirectGRPC(ctx, serviceHost.Host, logger, opts)
	if err != nil {
		return nil, err
	}
	return &ViamClient{conn: conn}, nil
}

func validateApiKeyFormat(apiKey string) (bool) {
	var regex = regexp.MustCompile("^[a-z0-9]{32}$")
	return regex.MatchString(apiKey)
}

func validateApiKeyIDFormat(apiKeyID string) (bool) {
	var regex = regexp.MustCompile("^[a-z0-9]{8}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{12}")
	return regex.MatchString(apiKeyID)
}
