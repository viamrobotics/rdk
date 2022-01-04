// Package main streams a specific camera over WebRTC.
package main

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/edaniels/gostream/codec/x264"
	"github.com/edaniels/gostream/media"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/camera/imagesource"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}
utils.ParseF
var (
	defaultPort = 5555
	logger      = golog.NewDevelopmentLogger("stream_camera")
)


func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var conf rimage.AttrConfig
	var mapArgs = args.(config.AttributeMap)
	argsP, err := config.TransformAttributeMapToStruct(&conf, mapArgs)
	if err != nil {
		return err
	}
	argsParsed := argsP.(rimage.AttrConfig)
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

	return viewCamera(ctx, argsParsed, logger)
}

func viewCamera(ctx context.Context, attrs rimage.AttrConfig, logger golog.Logger) error {
	webcam, err := imagesource.NewWebcamSource(&attrs, logger)
	if err != nil {
		return err
	}

	if err := func() error {
		img, closer, err := webcam.Next(ctx)
		if err != nil {
			return err
		}
		defer closer()
		if attrs.Debug {
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

	server, err := gostream.NewStandaloneStreamServer(attrs.Port, logger, remoteStream)
	if err != nil {
		return err
	}
	if err := server.Start(ctx); err != nil {
		return err
	}

	utils.ContextMainReadyFunc(ctx)()
	gostream.StreamSource(ctx, webcam, remoteStream)

	return server.Stop(ctx)
}
