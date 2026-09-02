package grpc

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.viam.com/utils"
	"go.viam.com/utils/grpchelpers"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/connectivity"

	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils"
	"go.viam.com/rdk/web/networkcheck"
)

// AppConn maintains an underlying client connection meant to be used globally to connect to App. The `AppConn` constructor repeatedly
// attempts to dial App until a connection is successfully established.
type AppConn struct {
	*ReconfigurableClientConn

	activeBackgroundWorkers *utils.StoppableWorkers
}

// NewAppConn creates an `AppConn` instance with a gRPC client connection to App. An initial dial attempt blocks. If it errors, the error
// is returned. If it times out, an `AppConn` object with a nil underlying client connection will return. Serialized attempts at
// establishing a connection to App will continue to occur, however, in a background Goroutine. These attempts will continue until a
// connection is made. If `cloud` is nil, an `AppConn` with a nil underlying connection will return, and the background dialer will not
// start.
func NewAppConn(ctx context.Context, appAddress, partID string, cloudCreds rpc.DialOption, logger logging.Logger) (rpc.ClientConn, error) {
	appConn := &AppConn{
		ReconfigurableClientConn: &ReconfigurableClientConn{Logger: logger.Sublogger("app_conn")},
		activeBackgroundWorkers:  utils.NewStoppableWorkers(ctx),
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
	appConn.conn, err = authedDialDirectGRPC(ctxWithTimeout, partID, grpcURL.Host, logger, dialOpts...)
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

	appConn.activeBackgroundWorkers.Add(func(ctx context.Context) {
		// Upon failing to dial app.viam.com, run DNS and packet loss checks asynchronously
		// to reveal more network information.
		networkcheck.TestDNS(ctx, logger, false /* non-verbose to only log failures */)
		networkcheck.TestPacketLoss(ctx, logger, false /* non-verbose to only log failures */)
	})
	appConn.activeBackgroundWorkers.Add(func(ctx context.Context) {
		for {
			if ctx.Err() != nil {
				return
			}

			ctxWithTimeout, ctxWithTimeoutCancel := contextutils.GetTimeoutCtx(ctx, false, partID, logger)
			conn, err := authedDialDirectGRPC(ctxWithTimeout, partID, grpcURL.Host, logger, dialOpts...)
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

func jwtInfo(partID, host string) (string, string) {
	cacheDir := filepath.Join(rutils.ViamDotDir, "grpc")
	jwtFilename := base64.RawURLEncoding.EncodeToString([]byte(partID + "_" + host))
	jwtPath := filepath.Join(cacheDir, jwtFilename+".jwt")

	return cacheDir, jwtPath
}

func jwtCacheWrite(partID, host, jwt string) error {
	cacheDir, jwtPath := jwtInfo(partID, host)

	err := os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		return err
	}

	// atomic write - this duplicates the atomic write in goutils so we don't
	// have to pull goutils.artifact and all its dependencies into grpc
	tmpFile, err := os.CreateTemp(cacheDir, "*.jwt.tmp")
	if err != nil {
		return err
	}
	_, err = tmpFile.WriteString(jwt)
	if err != nil {
		return errors.Join(err, tmpFile.Close(), os.Remove(tmpFile.Name()))
	}
	err = tmpFile.Close()
	if err != nil {
		return errors.Join(err, os.Remove(tmpFile.Name()))
	}
	err = os.Rename(tmpFile.Name(), jwtPath)
	if err != nil {
		return errors.Join(err, os.Remove(tmpFile.Name()))
	}
	return nil
}

func jwtCacheRead(partID, host string) (string, error) {
	_, jwtPath := jwtInfo(partID, host)

	// we need to write the JWT provided by app to disk
	//nolint:gosec
	jwtData, err := os.ReadFile(jwtPath)
	if err != nil {
		return "", err
	}
	return string(jwtData), nil
}

func redialWithCachedJwt(ctx context.Context, conn rpc.ClientConn, partID, host string,
	logger utils.ZapCompatibleLogger, dialOpts ...rpc.DialOption,
) rpc.ClientConn {
	cachedJwt, cacheErr := jwtCacheRead(partID, host)
	if cacheErr != nil {
		logger.Warnw("could not read JWT from cache", "error", cacheErr)
		// the connection is still valid, so return it
		return conn
	}
	// attempt to dial again with the cached credentials
	newConn, newErr := rpc.DialDirectGRPC(ctx, host, logger, append(dialOpts, rpc.WithStaticAuthenticationMaterial(cachedJwt))...)
	if newErr != nil {
		logger.Warnw(fmt.Sprintf("could not dial %s with cached JWT", host), "error", newErr)
		return conn
	}
	// the new connection with the cached token succeeded, so return it and close the old one
	err := conn.Close()
	if err != nil {
		logger.Warnw(fmt.Sprintf("could not close old connection with %s", host), "error", err)
	}
	return newConn
}

// authedDialDirectGRPC calls rpc.DialDirectGRPC while also explicitly authenticating the connection and caching the
// JWT (provided by app) on disk. If the normal auth path fails, we attempt to auth with the cached JWT
func authedDialDirectGRPC(ctx context.Context,
	partID, host string,
	logger utils.ZapCompatibleLogger,
	dialOpts ...rpc.DialOption,
) (rpc.ClientConn, error) {
	conn, err := rpc.DialDirectGRPC(ctx, host, logger, dialOpts...)
	if err != nil {
		return conn, err
	}
	authenticator, ok := conn.(rpc.ClientConnAuthenticator)
	if !ok {
		logger.Warnw("connection cannot be authenticated: robot did not supply cloud credentials", "connection", fmt.Sprintf("%T", conn))
		return conn, nil
	}
	freshJwt, authErr := authenticator.Authenticate(ctx)
	if authErr != nil {
		logger.Warnw(fmt.Sprintf("authenticating connection with %s failed, attempting to dial again with cached JWT", host), "error", authErr)
		return redialWithCachedJwt(ctx, conn, partID, host, logger, dialOpts...), nil
	}
	if freshJwt == "" {
		logger.Warnf("auth succeeded but the JWT from %s was empty, not updating cache", host)
	} else {
		cacheErr := jwtCacheWrite(partID, host, freshJwt)
		if cacheErr != nil {
			logger.Warnw("could not write to JWT cache", "error", cacheErr)
		}
	}
	return conn, nil
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

	ac.activeBackgroundWorkers.Add(func(ctx context.Context) {
		var offline bool
		state := checker.GetState()
		for {
			stateChanged, err := checker.WaitForStateChange(ctx, state)
			if err != nil {
				logger.Debugw("failed to subscribe to state changes; will not log lost/regained connections to app",
					"error", err.Error())
				return
			}
			if !stateChanged {
				// state did not change and ctx expired; AppConn is closing.
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
	ac.connMu.RLock()
	conn := ac.conn
	ac.connMu.RUnlock()

	checker, ok := conn.(grpchelpers.ConnectivityState)
	if !ok {
		return connectivity.Connecting
	}
	return checker.GetState()
}

// WaitForStateChange blocks until the connectivity state of the underlying connection
// changes from sourceState or ctx expires, returning true in the former case and false in
// the latter. If the underlying connection is nil or does not support state subscription,
// it returns false and an error.
func (ac *AppConn) WaitForStateChange(ctx context.Context, sourceState connectivity.State) (bool, error) {
	ac.connMu.RLock()
	conn := ac.conn
	ac.connMu.RUnlock()

	checker, ok := conn.(grpchelpers.ConnectivityState)
	if !ok {
		return false, errors.New("underlying connection does not allow waiting for state change")
	}
	return checker.WaitForStateChange(ctx, sourceState)
}

// Close attempts to close the underlying connection and stops background workers (dialing attempts and state watching).
func (ac *AppConn) Close() error {
	ac.activeBackgroundWorkers.Stop()

	return ac.ReconfigurableClientConn.Close()
}
