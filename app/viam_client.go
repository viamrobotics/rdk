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

func createViamClient(ctx context.Context, viamBaseURL string, opts rpc.DialOption, logger logging.Logger) (*viamClient, error) {
	if viamBaseURL == "" {
		viamBaseURL = "https://app.viam.com"
	}
	viamURL, err := url.Parse(viamBaseURL + ":443")
	if err != nil {
		return nil, err
	}

	conn, err := rpc.DialDirectGRPC(ctx, viamURL.Host, logger, opts)
	if err != nil {
		return nil, err
	}
	return &viamClient{conn: conn}, nil
}
