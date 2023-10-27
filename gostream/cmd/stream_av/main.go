// Package main streams audio and video.
package main

import (
	"context"

	"github.com/edaniels/golog"
	// register drivers.
	_ "github.com/pion/mediadevices/pkg/driver/camera"
	_ "github.com/pion/mediadevices/pkg/driver/microphone"
	_ "github.com/pion/mediadevices/pkg/driver/screen"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/gostream/codec/opus"
	"go.viam.com/rdk/gostream/codec/vpx"
)

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

var (
	defaultPort = 5555
	logger      = golog.Global().Named("server")
)

// Arguments for the command.
type Arguments struct {
	Port   goutils.NetPortFlag `flag:"0"`
	Camera bool                `flag:"camera,usage=use camera"`
	Dump   bool                `flag:"dump"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var argsParsed Arguments
	if err := goutils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Dump {
		allAudio := gostream.QueryAudioDevices()
		if len(allAudio) > 0 {
			logger.Debug("Audio:")
		}
		for _, info := range allAudio {
			logger.Debugf("%s", info.ID)
			logger.Debugf("\t labels: %v", info.Labels)
			logger.Debugf("\t priority: %v", info.Priority)
			for _, p := range info.Properties {
				logger.Debugf("\t %+v", p.Audio)
			}
		}
		var allVideo []gostream.DeviceInfo
		if argsParsed.Camera {
			allVideo = gostream.QueryVideoDevices()
		} else {
			allVideo = gostream.QueryScreenDevices()
		}
		if len(allVideo) > 0 {
			logger.Debug("Video:")
		}
		for _, info := range allVideo {
			logger.Debugf("%s", info.ID)
			logger.Debugf("\t labels: %v", info.Labels)
			logger.Debugf("\t priority: %v", info.Priority)
			for _, p := range info.Properties {
				logger.Debugf("\t %+v", p.Video)
			}
		}
		return nil
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = goutils.NetPortFlag(defaultPort)
	}

	return runServer(
		ctx,
		int(argsParsed.Port),
		argsParsed.Camera,
		logger,
	)
}

func runServer(
	ctx context.Context,
	port int,
	camera bool,
	logger golog.Logger,
) (err error) {
	audioSource, err := gostream.GetAnyAudioSource(gostream.DefaultConstraints, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, audioSource.Close(ctx))
	}()
	var videoSrc gostream.VideoSource
	if camera {
		videoSrc, err = gostream.GetAnyVideoSource(gostream.DefaultConstraints, logger)
	} else {
		videoSrc, err = gostream.GetAnyScreenSource(gostream.DefaultConstraints, logger)
	}
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, videoSrc.Close(ctx))
	}()

	var config gostream.StreamConfig
	config.AudioEncoderFactory = opus.NewEncoderFactory()
	config.VideoEncoderFactory = vpx.NewEncoderFactory(vpx.Version8)
	stream, err := gostream.NewStream(config)
	if err != nil {
		return err
	}
	server, err := gostream.NewStandaloneStreamServer(port, logger, nil, stream)
	if err != nil {
		return err
	}
	if err := server.Start(ctx); err != nil {
		return err
	}

	audioErr := make(chan error)
	defer func() {
		err = multierr.Combine(err, <-audioErr, server.Stop(ctx))
	}()

	go func() {
		audioErr <- gostream.StreamAudioSource(ctx, audioSource, stream)
	}()
	return gostream.StreamVideoSource(ctx, videoSrc, stream)
}
