// Package audioinput defines an audio capturing device.
package audioinput

import (
	"context"
	"errors"
	"sync"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	pb "go.viam.com/api/component/audioinput/v1"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.AudioInputService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterAudioInputServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.AudioInputService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	// TODO(RSDK-562): Add RegisterCollector
}

// SubtypeName is a constant that identifies the audio input resource subtype string.
const SubtypeName = resource.SubtypeName("audio_input")

// Subtype is a constant that identifies the audio input resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named audio inputs's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// An AudioInput represents anything that can capture audio.
type AudioInput interface {
	gostream.AudioSource
	gostream.AudioPropertyProvider
	generic.Generic
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((AudioInput)(nil), actual)
}

// DependencyTypeError is used when a resource doesn't implement the expected interface.
func DependencyTypeError(name, actual interface{}) error {
	return utils.DependencyTypeError(name, (AudioInput)(nil), actual)
}

// WrapWithReconfigurable wraps an audio input with a reconfigurable and locking interface.
func WrapWithReconfigurable(r interface{}, name resource.Name) (resource.Reconfigurable, error) {
	i, ok := r.(AudioInput)
	if !ok {
		return nil, NewUnimplementedInterfaceError(r)
	}
	if reconfigurable, ok := i.(*reconfigurableAudioInput); ok {
		return reconfigurable, nil
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	return &reconfigurableAudioInput{
		name:      name,
		actual:    i,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}, nil
}

var (
	_ = AudioInput(&reconfigurableAudioInput{})
	_ = resource.Reconfigurable(&reconfigurableAudioInput{})
	_ = viamutils.ContextCloser(&reconfigurableAudioInput{})
)

// FromDependencies is a helper for getting the named audio input from a collection of
// dependencies.
func FromDependencies(deps registry.Dependencies, name string) (AudioInput, error) {
	res, ok := deps[Named(name)]
	if !ok {
		return nil, utils.DependencyNotFoundError(name)
	}
	part, ok := res.(AudioInput)
	if !ok {
		return nil, DependencyTypeError(name, res)
	}
	return part, nil
}

// FromRobot is a helper for getting the named audio input from the given Robot.
func FromRobot(r robot.Robot, name string) (AudioInput, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(AudioInput)
	if !ok {
		return nil, NewUnimplementedInterfaceError(res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all audio input names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

type audioPropertiesFunc func(ctx context.Context) (prop.Audio, error)

func (apf audioPropertiesFunc) MediaProperties(ctx context.Context) (prop.Audio, error) {
	return apf(ctx)
}

// NewFromReader creates an AudioInput from a reader.
func NewFromReader(reader gostream.AudioReader, props prop.Audio) (AudioInput, error) {
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

// NewFromSource creates an AudioInput from an AudioSource.
func NewFromSource(audSrc gostream.AudioSource) (AudioInput, error) {
	if audSrc == nil {
		return nil, errors.New("cannot have a nil audio source")
	}
	provider, ok := audSrc.(gostream.AudioPropertyProvider)
	if !ok {
		return nil, errors.New("source must have property provider")
	}
	return &audioSource{as: audSrc, prov: provider}, nil
}

// AudioSource implements an AudioInput with a gostream.AudioSource.
type audioSource struct {
	as   gostream.AudioSource
	prov gostream.AudioPropertyProvider
	generic.Unimplemented
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

func (as *audioSource) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if doer, ok := as.as.(generic.Generic); ok {
		return doer.DoCommand(ctx, cmd)
	}
	return nil, generic.ErrUnimplemented
}

// Close closes the underlying AudioSource.
func (as *audioSource) Close(ctx context.Context) error {
	return viamutils.TryClose(ctx, as.as)
}

type reconfigurableAudioInput struct {
	mu        sync.RWMutex
	name      resource.Name
	actual    AudioInput
	cancelCtx context.Context
	cancel    func()
}

func (i *reconfigurableAudioInput) Name() resource.Name {
	return i.name
}

func (i *reconfigurableAudioInput) ProxyFor() interface{} {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.actual
}

func (i *reconfigurableAudioInput) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.AudioStream, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	stream := &reconfigurableAudioStream{
		i:           i,
		errHandlers: errHandlers,
		cancelCtx:   i.cancelCtx,
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if err := stream.init(ctx); err != nil {
		return nil, err
	}

	return stream, nil
}

type reconfigurableAudioStream struct {
	mu          sync.Mutex
	i           *reconfigurableAudioInput
	errHandlers []gostream.ErrorHandler
	stream      gostream.AudioStream
	cancelCtx   context.Context
}

func (as *reconfigurableAudioStream) init(ctx context.Context) error {
	var err error
	as.stream, err = as.i.actual.Stream(ctx, as.errHandlers...)
	return err
}

func (as *reconfigurableAudioStream) Next(ctx context.Context) (wave.Audio, func(), error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.stream == nil || as.cancelCtx.Err() != nil {
		if err := func() error {
			as.i.mu.Lock()
			defer as.i.mu.Unlock()
			return as.init(ctx)
		}(); err != nil {
			return nil, nil, err
		}
	}
	return as.stream.Next(ctx)
}

func (as *reconfigurableAudioStream) Close(ctx context.Context) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if as.stream == nil {
		return nil
	}
	return as.stream.Close(ctx)
}

func (i *reconfigurableAudioInput) MediaProperties(ctx context.Context) (prop.Audio, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.actual.MediaProperties(ctx)
}

func (i *reconfigurableAudioInput) Close(ctx context.Context) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.actual.Close(ctx)
}

func (i *reconfigurableAudioInput) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.actual.DoCommand(ctx, cmd)
}

// Reconfigure reconfigures the resource.
func (i *reconfigurableAudioInput) Reconfigure(ctx context.Context, newAudioInput resource.Reconfigurable) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	actual, ok := newAudioInput.(*reconfigurableAudioInput)
	if !ok {
		return utils.NewUnexpectedTypeError(i, newAudioInput)
	}
	if err := viamutils.TryClose(ctx, i.actual); err != nil {
		golog.Global().Errorw("error closing old", "error", err)
	}
	i.cancel()
	// reset
	i.actual = actual.actual
	i.cancelCtx = actual.cancelCtx
	i.cancel = actual.cancel
	return nil
}

// UpdateAction helps hint the reconfiguration process on what strategy to use given a modified config.
// See config.ShouldUpdateAction for more information.
func (i *reconfigurableAudioInput) UpdateAction(conf *config.Component) config.UpdateActionType {
	obj, canUpdate := i.actual.(config.ComponentUpdate)
	if canUpdate {
		return obj.UpdateAction(conf)
	}
	return config.Reconfigure
}
