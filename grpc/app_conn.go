package grpc

import (
	"context"
	"net/url"

	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

// spin off Goroutine that attempts to create connection
// routine should at first block for some time interval
// if connection is not created after initial timeout, no longer block
// however, continue re-attempting connection at other specified time interval
// once connection establishes, close off routine

type AppConn struct {
	ReconfigurableClientConn
}

func CreateNewGRPCClient(ctx context.Context, cloudCfg *logging.CloudConfig, logger logging.Logger) (rpc.ClientConn, error) {
	grpcURL, err := url.Parse(cloudCfg.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)
	// Only add credentials when secret is set.
	if cloudCfg.Secret != "" {
		dialOpts = append(dialOpts, rpc.WithEntityCredentials(cloudCfg.ID,
			rpc.Credentials{
				Type:    "robot-secret",
				Payload: cloudCfg.Secret,
			},
		))
	}

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	return rpc.DialDirectGRPC(ctx, grpcURL.Host, logger, dialOpts...)
}
