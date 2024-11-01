package app

import (
	"context"
	"net/url"

	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

type viamClient struct {
	conn rpc.ClientConn
}

func createViamClient(ctx context.Context, baseURL string, opts rpc.DialOption, logger logging.Logger) (*viamClient, error) {
	if baseURL == "" {
		baseURL = "https://app.viam.com"
	}
	serviceHost, err := url.Parse(baseURL + ":443")
	if err != nil {
		return nil, err
	}

	conn, err := rpc.DialDirectGRPC(ctx, serviceHost.Host, logger, opts)
	if err != nil {
		return nil, err
	}
	return &viamClient{conn: conn}, nil
}
