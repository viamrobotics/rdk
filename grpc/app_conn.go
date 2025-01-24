package grpc

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
)

type AppConn struct {
	ReconfigurableClientConn
}

func NewAppConn(ctx context.Context, cloud *config.Cloud, logger logging.Logger) (*AppConn, error) {
	grpcURL, err := url.Parse(cloud.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := dialOpts(cloud)

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	appConn := &AppConn{}

	appConn.connMu.Lock()
	defer appConn.connMu.Unlock()
	appConn.conn, err = rpc.DialDirectGRPC(ctx, grpcURL.Host, logger, dialOpts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// TODO(RSDK-8292): run background job to attempt connection
		} else {
			return nil, err
		}
	}

	return appConn, nil
}

func dialOpts(cloud *config.Cloud) []rpc.DialOption {
	dialOpts := make([]rpc.DialOption, 0, 2)
	// Only add credentials when secret is set.
	if cloud.Secret != "" {
		dialOpts = append(dialOpts, rpc.WithEntityCredentials(cloud.ID,
			rpc.Credentials{
				Type:    "robot-secret",
				Payload: cloud.Secret,
			},
		))
	}
	return dialOpts
}
