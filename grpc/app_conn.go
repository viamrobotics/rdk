package grpc

import (
	"context"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/utils/rpc"
)

type AppConn struct {
	ReconfigurableClientConn

	// Err stores the most recent error returned by the serialized dial attempts running in the background. It can also be used to tell
	// whether dial attempts are currently happening; If err is a non-nil value, dial attempts have stopped. Accesses to Err should respect
	// ReconfigurableClientConn.connMu
	Err error
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

	// a lock is not necessary here because this call is blocking
	appConn.conn, err = rpc.DialDirectGRPC(ctx, grpcURL.Host, logger, dialOpts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			go func() {
				for {
					appConn.connMu.Lock()

					// TODO(RSDK-8292): [qu] should I use ctx instead of context.Background()
					ctxWithTimeOut, ctxWithTimeOutCancel := context.WithTimeout(context.Background(), 5*time.Second)

					appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeOut, grpcURL.Host, logger, dialOpts...)
					if errors.Is(err, context.DeadlineExceeded) {
						appConn.connMu.Unlock()

						// only dial again if previous attempt timed out
						continue
					}

					ctxWithTimeOutCancel()

					appConn.Err = err

					break
				}

				appConn.connMu.Unlock()
			}()
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
