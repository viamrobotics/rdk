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
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*Attrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newMicrophoneSource(attrs, logger)
		}})

	config.RegisterComponentAttributeMapConverter(audioinput.Subtype, model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Attrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*Attrs)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		}, &Attrs{})
}

// Attrs is the attribute struct for microphones.
type Attrs struct {
	Path        string `json:"audio_path"`
	PathPattern string `json:"audio_path_pattern"`
	Debug       bool   `json:"debug"`
}

// newMicrophoneSource returns a new source based on a microphone discovered from the given attributes.
func newMicrophoneSource(attrs *Attrs, logger golog.Logger) (audioinput.AudioInput, error) {
	var err error

	debug := attrs.Debug

	if attrs.Path != "" {
		return tryMicrophoneOpen(attrs.Path, gostream.DefaultConstraints, logger)
	}

	var pattern *regexp.Regexp
	if attrs.PathPattern != "" {
		pattern, err = regexp.Compile(attrs.PathPattern)
		if err != nil {
			return nil, err
		}
	}
	all := gostream.QueryAudioDevices()

	for i, info := range all {
		logger.Debugf("%s", info.ID)
		logger.Debugf("\t labels: %v", info.Labels)
		if pattern != nil && !pattern.MatchString(info.Labels[i]) {
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
			s, err := tryMicrophoneOpen(info.Labels[i], gostream.DefaultConstraints, logger)
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
	return nil, errors.New("found no microphones")
}

func tryMicrophoneOpen(
	path string,
	constraints mediadevices.MediaStreamConstraints,
	logger golog.Logger,
) (audioinput.AudioInput, error) {
	source, err := gostream.GetNamedAudioSource(filepath.Base(path), constraints, logger)
	if err != nil {
		return nil, err
	}
	// TODO(XXX): implement LivenessMonitor
	return audioinput.NewFromSource(source)
}
