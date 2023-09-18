// Package audioinput defines an audio capturing device.
package audioinput

import (
	"context"
	"errors"

	"github.com/pion/mediadevices/pkg/prop"
	pb "go.viam.com/api/component/audioinput/v1"
	"go.viam.com/rdk/gostream"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[AudioInput]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterAudioInputServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.AudioInputService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})

	// TODO(RSDK-562): Add RegisterCollector
}

// SubtypeName is a constant that identifies the audio input resource subtype string.
const SubtypeName = "audio_input"

// API is a variable that identifies the audio input resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named audio inputs's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// An AudioInput is a resource that can capture audio.
type AudioInput interface {
	resource.Resource
	AudioSource
}

// An AudioSource represents anything that can capture audio.
type AudioSource interface {
	gostream.AudioSource
	gostream.AudioPropertyProvider
}

// A LivenessMonitor is responsible for monitoring the liveness of an audio input. An example
// is connectivity. Since the model itself knows best about how to maintain this state,
// the reconfigurable offers a safe way to notify if a state needs to be reset due
// to some exceptional event (like a reconnect).
// It is expected that the monitoring code is tied to the lifetime of the resource
// and once the resource is closed, so should the monitor. That is, it should
// no longer send any resets once a Close on its associated resource has returned.
type LivenessMonitor interface {
	Monitor(notifyReset func())
}

// FromDependencies is a helper for getting the named audio input from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (AudioInput, error) {
	return resource.FromDependencies[AudioInput](deps, Named(name))
}

// FromRobot is a helper for getting the named audio input from the given Robot.
func FromRobot(r robot.Robot, name string) (AudioInput, error) {
	return robot.ResourceFromRobot[AudioInput](r, Named(name))
}

// NamesFromRobot is a helper for getting all audio input names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

type audioPropertiesFunc func(ctx context.Context) (prop.Audio, error)

func (apf audioPropertiesFunc) MediaProperties(ctx context.Context) (prop.Audio, error) {
	return apf(ctx)
}

// NewAudioSourceFromReader creates an AudioSource from a reader.
func NewAudioSourceFromReader(reader gostream.AudioReader, props prop.Audio) (AudioSource, error) {
	if reader == nil {
		return nil, errors.New("cannot have a nil reader")
	}
	as := gostream.NewAudioSource(reader, props)
	return &audioSource{
		as: as,
		prov: audioPropertiesFunc(func(ctx context.Context) (prop.Audio, error) {
			return props, nil
		}),
	}, nil
}

// FromAudioSource creates an AudioInput resource either from a AudioSource.
func FromAudioSource(name resource.Name, src AudioSource) (AudioInput, error) {
	return &sourceBasedInput{
		Named:       name.AsNamed(),
		AudioSource: src,
	}, nil
}

type sourceBasedInput struct {
	resource.Named
	resource.AlwaysRebuild
	AudioSource
}

// NewAudioSourceFromGostreamSource creates an AudioSource from a gostream.AudioSource.
func NewAudioSourceFromGostreamSource(audSrc gostream.AudioSource) (AudioSource, error) {
	if audSrc == nil {
		return nil, errors.New("cannot have a nil audio source")
	}
	provider, ok := audSrc.(gostream.AudioPropertyProvider)
	if !ok {
		return nil, errors.New("source must have property provider")
	}
	return &audioSource{
		as:   audSrc,
		prov: provider,
	}, nil
}

// AudioSource implements an AudioInput with a gostream.AudioSource.
type audioSource struct {
	as   gostream.AudioSource
	prov gostream.AudioPropertyProvider
}

func (as *audioSource) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.AudioStream, error) {
	return as.as.Stream(ctx, errHandlers...)
}

func (as *audioSource) MediaProperties(ctx context.Context) (prop.Audio, error) {
	return as.prov.MediaProperties(ctx)
}

// Close closes the underlying AudioSource.
func (as *audioSource) Close(ctx context.Context) error {
	return as.as.Close(ctx)
}
