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

type Options struct {
	baseURL string
	entity string
	credentials rpc.Credentials
}

var dialDirectGRPC = rpc.DialDirectGRPC

// CreateViamClientWithOptions creates a ViamClient with an Options struct
func CreateViamClientWithOptions(ctx context.Context, options Options, logger logging.Logger) (*ViamClient, error) {
	if options.baseURL == "" {
		options.baseURL = "https://app.viam.com"
	} else if !strings.HasPrefix(options.baseURL, "http://") && !strings.HasPrefix(options.baseURL, "https://") {
		return nil, errors.New("use valid URL")
	}
	serviceHost, err := url.Parse(options.baseURL + ":443")
	if err != nil {
		return nil, err
	}

	if options.credentials.Payload == "" || options.entity == "" {
		return nil, errors.New("entity and payload cannot be empty")
	}
	opts := rpc.WithEntityCredentials(options.entity, options.credentials)

	conn, err := dialDirectGRPC(ctx, serviceHost.Host, logger, opts)
	if err != nil {
		return nil, err
	}
	return &ViamClient{conn: conn}, nil
}

// Close closes the gRPC connection.
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
