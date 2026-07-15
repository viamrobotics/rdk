package grpc

import (
	"context"
	"net/url"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/grpchelpers"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/connectivity"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/contextutils"
	"go.viam.com/rdk/web/networkcheck"
)

// AppConn maintains an underlying client connection meant to be used globally to connect to App. The `AppConn` constructor repeatedly
// attempts to dial App until a connection is successfully established.
type AppConn struct {
	*ReconfigurableClientConn

	dialer *utils.StoppableWorkers

	stateWatcher *utils.StoppableWorkers
}

// NewAppConn creates an `AppConn` instance with a gRPC client connection to App. An initial dial attempt blocks. If it errors, the error
// is returned. If it times out, an `AppConn` object with a nil underlying client connection will return. Serialized attempts at
// establishing a connection to App will continue to occur, however, in a background Goroutine. These attempts will continue until a
// connection is made. If `cloud` is nil, an `AppConn` with a nil underlying connection will return, and the background dialer will not
// start.
func NewAppConn(ctx context.Context, appAddress, partID string, cloudCreds rpc.DialOption, logger logging.Logger) (rpc.ClientConn, error) {
	appConn := &AppConn{
		ReconfigurableClientConn: &ReconfigurableClientConn{Logger: logger.Sublogger("app_conn")},
		stateWatcher:             utils.NewStoppableWorkers(ctx),
	}

	grpcURL, err := url.Parse(appAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)

	if cloudCreds != nil {
		dialOpts = append(dialOpts, cloudCreds)
	}

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	ctxWithTimeout, ctxWithTimeoutCancel := contextutils.GetTimeoutCtx(ctx, true, partID, logger)
	defer ctxWithTimeoutCancel()
	// there will always be a deadline
	if deadline, ok := ctxWithTimeout.Deadline(); ok {
		logger.CInfow(
			ctx,
			"attempting to establish initial global connection to app",
			"url",
			grpcURL.Host,
			"start_time",
			time.Now().String(),
			"deadline",
			deadline.String(),
		)
	}

	// lock not necessary here because call is blocking
	appConn.conn, err = rpc.DialDirectGRPC(ctxWithTimeout, grpcURL.Host, logger, dialOpts...)
	if err == nil {
		appConn.watchState(grpcURL.Host, logger)
		return appConn, nil
	}
	logger.CInfow(
		ctx,
		"failed to establish initial global connection to app, starting background worker to establish connection...",
		"url",
		grpcURL.Host,
		"error",
		err,
	)

	appConn.dialer = utils.NewStoppableWorkers(ctx)
	appConn.dialer.Add(func(ctx context.Context) {
		// Upon failing to dial app.viam.com, run DNS and packet loss checks asynchronously
		// to reveal more network information.
		networkcheck.TestDNS(ctx, logger, false /* non-verbose to only log failures */)
		networkcheck.TestPacketLoss(ctx, logger, false /* non-verbose to only log failures */)
	})
	appConn.dialer.Add(func(ctx context.Context) {
		for {
			if ctx.Err() != nil {
				return
			}

			ctxWithTimeout, ctxWithTimeoutCancel := contextutils.GetTimeoutCtx(ctx, false, partID, logger)
			conn, err := rpc.DialDirectGRPC(ctxWithTimeout, grpcURL.Host, logger, dialOpts...)
			ctxWithTimeoutCancel()
			if err != nil {
				logger.Debugw("error while dialing app. Could not establish global, unified connection", "error", err)

				continue
			}
			logger.CInfow(ctx, "successfully established global connection to app", "url", grpcURL.Host)
			appConn.connMu.Lock()
			appConn.conn = conn
			appConn.connMu.Unlock()
			appConn.watchState(grpcURL.Host, logger)

			return
		}
	})

	return appConn, nil
}

// watchState starts a background worker that subscribes to connectivity state changes on
// the underlying connection and logs when the connection to App is lost or regained. A
// connection is considered lost when its state moves to TransientFailure or Connecting
// and regained when it moves back to Ready. The worker spends its life blocked in
// `WaitForStateChange` and does no polling.
func (ac *AppConn) watchState(host string, logger logging.Logger) {
	ac.connMu.RLock()
	conn := ac.conn
	ac.connMu.RUnlock()

	checker, ok := conn.(grpchelpers.ConnectivityState)
	if !ok {
		logger.Debugw("connection does not support state subscription; will not log lost/regained connections to app",
			"url", host)
		return
	}

	ac.stateWatcher.Add(func(ctx context.Context) {
		var offline bool
		state := checker.GetState()
		for {
			if !checker.WaitForStateChange(ctx, state) {
				// ctx expired; AppConn is closing.
				return
			}
			state = checker.GetState()
			switch {
			case !offline && (state == connectivity.TransientFailure || state == connectivity.Connecting):
				offline = true
				logger.Infow("Lost connection to app", "url", host)
			case offline && state == connectivity.Ready:
				offline = false
				logger.Infow("Regained connection to app", "url", host)
			}
		}
	})
}

// GetState returns the current state of the connection.
func (ac *AppConn) GetState() connectivity.State {
	if ac.conn == nil {
		return connectivity.Connecting
	}

	checker, ok := ac.conn.(grpchelpers.ConnectivityState)
	if !ok {
		return connectivity.Connecting
	}
	return checker.GetState()
}

// Close attempts to close the underlying connection and stops background dialing attempts.
func (ac *AppConn) Close() error {
	if ac.dialer != nil {
		ac.dialer.Stop()
	}

	if ac.stateWatcher != nil {
		ac.stateWatcher.Stop()
	}

	return ac.ReconfigurableClientConn.Close()
}
