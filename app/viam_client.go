// Package app contains all logic needed for communication and interaction with app.
package app

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
)

// ViamClient is a gRPC client for method calls to Viam app.
type ViamClient struct {
	conn rpc.ClientConn
	appClient Client
}

// Options has the options necessary to connect through gRPC.
type Options struct {
	baseURL     string
	entity      string
	credentials rpc.Credentials
}

var dialDirectGRPC = rpc.DialDirectGRPC

// CreateViamClientWithOptions creates a ViamClient with an Options struct.
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
	return &ViamClient{
		conn: conn,
		appClient: NewClientFromConn(conn, logger),
	}, nil
}

// CreateViamClientWithAPIKey creates a ViamClient with an API key.
func CreateViamClientWithAPIKey(
	ctx context.Context, options Options, apiKey, apiKeyID string, logger logging.Logger,
) (*ViamClient, error) {
	options.entity = apiKeyID
	options.credentials = rpc.Credentials{
		Type:    rpc.CredentialsTypeAPIKey,
		Payload: apiKey,
	}
	return CreateViamClientWithOptions(ctx, options, logger)
}

// Close closes the gRPC connection.
func (c *ViamClient) Close() error {
	return c.conn.Close()
}
