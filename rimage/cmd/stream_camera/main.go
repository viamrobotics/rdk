// Package main streams a specific camera over WebRTC.
package main

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/core/component/camera/imagesource"
	"go.viam.com/core/config"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/edaniels/gostream/media"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 5555
	logger      = golog.NewDevelopmentLogger("stream_camera")
)

// Arguments for the command.
type Arguments struct {
	Port        utils.NetPortFlag `flag:"0"`
	Debug       bool              `flag:"debug"`
	Dump        bool              `flag:"dump,usage=dump all camera info"`
	Format      string            `flag:"format"`
	Path        string            `flag:"path"`
	PathPattern string            `flag:"pathPattern"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = utils.NetPortFlag(defaultPort)
	}

	if argsParsed.Dump {
		all := media.QueryVideoDevices()
		for _, info := range all {
			logger.Debugf("%s", info.ID)
			logger.Debugf("\t labels: %v", info.Labels)
			for _, p := range info.Properties {
				logger.Debugf("\t %v %d x %d", p.FrameFormat, p.Width, p.Height)
			}

		}
		return nil
	}

	attrs := config.AttributeMap{}

	if argsParsed.Format != "" {
		attrs["format"] = argsParsed.Format
	}

	if argsParsed.Path != "" {
		attrs["path"] = argsParsed.Path
	}

	if argsParsed.PathPattern != "" {
		attrs["path_pattern"] = argsParsed.PathPattern
	}

	if argsParsed.Debug {
		attrs["debug"] = true
	}

	if argsParsed.Debug {
		logger.Debugf("attrs: %v", attrs)
	}

	return viewCamera(ctx, attrs, int(argsParsed.Port), argsParsed.Debug, logger)
}

func viewCamera(ctx context.Context, attrs config.AttributeMap, port int, debug bool, logger golog.Logger) error {
	webcam, err := imagesource.NewWebcamSource(attrs, logger)
	if err != nil {
		return err
	}

	if err := func() error {
		img, closer, err := webcam.Next(ctx)
		if err != nil {
			return err
		}
		defer closer()
		if debug {
			logger.Debugf("image type: %T dimensions: %v", img, img.Bounds())
		}
		return nil
	}(); err != nil {
		return err
	}

	remoteStream, err := gostream.NewStream(x264.DefaultStreamConfig)
	if err != nil {
		return err
	}

	server, err := gostream.NewStandaloneStreamServer(port, logger, remoteStream)
	if err != nil {
		return err
	}
	if err := server.Start(ctx); err != nil {
		return err
	}

	utils.ContextMainReadyFunc(ctx)()
	gostream.StreamSource(ctx, webcam, remoteStream)

	return server.Stop(context.Background())
}
