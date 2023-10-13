//go:build !no_media

// Package server implements the entry point for running a robot web server.
package server

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera/videosource/logging"
)

func startVideoLogger(ctx context.Context) {
	utils.UncheckedError(logging.GLoggerCamComp.Start(ctx))
}
