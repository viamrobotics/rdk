// Package server implements the entry point for running a robot web server.
package server

import (
	"context"
	"time"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

const (
	defaultNeedsRestartCheckInterval = time.Second * 5
	minNeedsRestartCheckInterval     = time.Second * 1
)

type needsRestartChecker interface {
	needsRestart(ctx context.Context) (bool, time.Duration, error)
}

type needsRestartCheckerGRPC struct {
	cfg    *config.Cloud
	logger logging.Logger
	client rpc.ClientConn
}

func (c *needsRestartCheckerGRPC) needsRestart(ctx context.Context) (bool, time.Duration, error) {
	service := apppb.NewRobotServiceClient(c.client)
	res, err := service.NeedsRestart(ctx, &apppb.NeedsRestartRequest{Id: c.cfg.ID})
	if err != nil {
		return false, defaultNeedsRestartCheckInterval, err
	}

	restartInterval := res.RestartCheckInterval.AsDuration()
	if restartInterval < minNeedsRestartCheckInterval {
		c.logger.Warnf("received restart interval less than %s not using was %d",
			minNeedsRestartCheckInterval,
			res.RestartCheckInterval.AsDuration())
		restartInterval = defaultNeedsRestartCheckInterval
	}

	return res.MustRestart, restartInterval, nil
}

func newRestartChecker(cfg *config.Cloud, logger logging.Logger, client rpc.ClientConn) needsRestartChecker {
	return &needsRestartCheckerGRPC{
		cfg:    cfg,
		logger: logger,
		client: client,
	}
}
