// Package server implements the entry point for running a robot web server.
package server

import (
	"bytes"
	"context"
<<<<<<< HEAD
	"io/ioutil"
=======
	"io"
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
	"net/http"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	apppb "go.viam.com/api/proto/viam/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
)

const (
<<<<<<< HEAD
	defaultNeedsRestartCheckInternval = time.Second * 1
	minNeedsRestartCheckInternval     = time.Second * 1
=======
	defaultNeedsRestartCheckInterval = time.Second * 1
	minNeedsRestartCheckInterval     = time.Second * 1
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
)

type needsRestartChecker interface {
	needsRestart(ctx context.Context) (bool, time.Duration, error)
	close()
}

type needsRestartCheckerHTTP struct {
	cfg             *config.Cloud
	restartInterval time.Duration
	logger          golog.Logger
	client          http.Client
}

func (c *needsRestartCheckerHTTP) close() {
	c.client.CloseIdleConnections()
}

func (c *needsRestartCheckerHTTP) needsRestart(ctx context.Context) (bool, time.Duration, error) {
	req, err := config.CreateCloudRequest(ctx, c.cfg)
	if err != nil {
		return false, c.restartInterval, errors.Wrapf(err, "error creating cloud request")
	}
	req.URL.Path = "/api/json1/needs_restart"
	resp, err := c.client.Do(req)
	if err != nil {
		return false, c.restartInterval, errors.Wrapf(err, "error querying cloud request")
	}

	defer utils.UncheckedErrorFunc(resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return false, c.restartInterval, errors.Wrapf(err, "bad status code")
	}

<<<<<<< HEAD
	read, err := ioutil.ReadAll(resp.Body)
=======
	read, err := io.ReadAll(resp.Body)
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
	if err != nil {
		return false, c.restartInterval, errors.Wrapf(err, "failed to read body")
	}

	mustRestart := bytes.Equal(read, []byte("true"))
	return mustRestart, c.restartInterval, nil
}

type needsRestartCheckerGRPC struct {
	cfg    *config.Cloud
	logger golog.Logger
	client rpc.ClientConn
}

func (c *needsRestartCheckerGRPC) close() {
<<<<<<< HEAD
	if c.client != nil {
		utils.UncheckedErrorFunc(c.client.Close)
	}
=======
	utils.UncheckedErrorFunc(c.client.Close)
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
}

func (c *needsRestartCheckerGRPC) needsRestart(ctx context.Context) (bool, time.Duration, error) {
	service := apppb.NewRobotServiceClient(c.client)
	res, err := service.NeedsRestart(ctx, &apppb.NeedsRestartRequest{Id: c.cfg.ID})
	if err != nil {
<<<<<<< HEAD
		return false, defaultNeedsRestartCheckInternval, err
	}

	restartInterval := res.RestartCheckInterval.AsDuration()
	if restartInterval < minNeedsRestartCheckInternval {
		c.logger.Warnf("received restart interval less than 1 second not using was %d", res.RestartCheckInterval.AsDuration())
		restartInterval = defaultNeedsRestartCheckInternval
=======
		return false, defaultNeedsRestartCheckInterval, err
	}

	restartInterval := res.RestartCheckInterval.AsDuration()
	if restartInterval < minNeedsRestartCheckInterval {
		c.logger.Warnf("received restart interval less than %s not using was %d",
			minNeedsRestartCheckInterval,
			res.RestartCheckInterval.AsDuration())
		restartInterval = defaultNeedsRestartCheckInterval
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
	}

	return res.MustRestart, restartInterval, nil
}

func newRestartChecker(ctx context.Context, cfg *config.Cloud, logger golog.Logger) (needsRestartChecker, error) {
	if cfg.AppAddress == "" {
		return &needsRestartCheckerHTTP{
			cfg:             cfg,
			logger:          logger,
<<<<<<< HEAD
			restartInterval: defaultNeedsRestartCheckInternval,
=======
			restartInterval: defaultNeedsRestartCheckInterval,
>>>>>>> d4751676 ([APP-531] Optionally use gRPC endpoint to check if robot needs to restart (#1179))
			client:          http.Client{},
		}, nil
	}

	client, err := config.CreateNewGRPCClient(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}

	return &needsRestartCheckerGRPC{
		cfg:    cfg,
		logger: logger,
		client: client,
	}, nil
}
