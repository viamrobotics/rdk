// Package microphone implements a microphone audio input. Really the microphone
// is any audio input device that can be found via gostream.
package microphone

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices"

	"go.viam.com/rdk/components/audioinput"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("microphone")

func init() {
	registry.RegisterComponent(
		audioinput.Subtype,
		model,
		registry.Component{Constructor: func(
			_ context.Context,
			_ resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			newConf, err := resource.NativeConfig[*Config](conf)
			if err != nil {
				return nil, err
			}
			src, err := newMicrophoneSource(newConf, logger)
			if err != nil {
				return nil, err
			}
			// This always rebuilds on reconfiguration right now. A better system
			// would be to reuse the monitored webcam code.
			return audioinput.FromAudioSource(conf.ResourceName(), src)
		}})

	config.RegisterComponentAttributeMapConverter(audioinput.Subtype, model,
		func(attributes utils.AttributeMap) (interface{}, error) {
			return config.TransformAttributeMapToStruct(&Config{}, attributes)
		})
}

// Config is the attribute struct for microphones.
type Config struct {
	Path        string `json:"audio_path"`
	PathPattern string `json:"audio_path_pattern"`
	Debug       bool   `json:"debug"`
}

// newMicrophoneSource returns a new source based on a microphone discovered from the given attributes.
func newMicrophoneSource(conf *Config, logger golog.Logger) (audioinput.AudioSource, error) {
	var err error

	debug := conf.Debug

	if conf.Path != "" {
		return tryMicrophoneOpen(conf.Path, gostream.DefaultConstraints, logger)
	}

	var pattern *regexp.Regexp
	if conf.PathPattern != "" {
		pattern, err = regexp.Compile(conf.PathPattern)
		if err != nil {
			return nil, err
		}
	}
	all := gostream.QueryAudioDevices()

	for _, info := range all {
		logger.Debugf("%s", info.ID)
		logger.Debugf("\t labels: %v", info.Labels)
		for _, label := range info.Labels {
			if pattern != nil && !pattern.MatchString(label) {
				if debug {
					logger.Debug("\t skipping because of pattern")
				}
				continue
			}
			for _, p := range info.Properties {
				logger.Debugf("\t %+v", p.Audio)
				if p.Audio.ChannelCount == 0 {
					if debug {
						logger.Debug("\t skipping because audio channels are empty")
					}
					continue
				}
				s, err := tryMicrophoneOpen(label, gostream.DefaultConstraints, logger)
				if err == nil {
					if debug {
						logger.Debug("\t USING")
					}
					return s, nil
				}
				if debug {
					logger.Debugw("cannot open driver with properties", "properties", p,
						"error", err)
				}
			}
		}
	}
	return nil, errors.New("found no microphones")
}

func tryMicrophoneOpen(
	path string,
	constraints mediadevices.MediaStreamConstraints,
	logger golog.Logger,
) (audioinput.AudioSource, error) {
	source, err := gostream.GetNamedAudioSource(filepath.Base(path), constraints, logger)
	if err != nil {
		return nil, err
	}
	// TODO(XXX): implement LivenessMonitor
	return audioinput.NewAudioSourceFromGostreamSource(source)
}
