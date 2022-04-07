// Package robotutils are a collection of util methods for creating and running robots in rdk
package robotutils

import (
	"context"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/web"
)

// RunWeb starts the web server on the web service with web options and blocks until we close it.
func RunWeb(ctx context.Context, r robot.Robot, o web.Options, logger golog.Logger) (err error) {
	defer func() {
		if err != nil {
			err = utils.FilterOutError(err, context.Canceled)
			if err != nil {
				logger.Errorw("error running web", "error", err)
			}
		}
		err = multierr.Combine(err, utils.TryClose(ctx, r))
	}()
	svc, err := web.FromRobot(r)
	if err != nil {
		return err
	}
	if err := svc.Start(ctx, o); err != nil {
		return err
	}
	<-ctx.Done()
	return ctx.Err()
}

// RunWebWithConfig starts the web server on the web service with a robot config and blocks until we close it.
func RunWebWithConfig(ctx context.Context, r robot.Robot, cfg *config.Config, logger golog.Logger) error {
	o, err := web.OptionsFromConfig(cfg)
	if err != nil {
		return err
	}
	return RunWeb(ctx, r, o, logger)
}
