// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/pexec"

	// registers all components.
	_ "go.viam.com/rdk/component/register"
	"go.viam.com/rdk/config"

	// register vm engines.
	_ "go.viam.com/rdk/function/vm/engines/javascript"
	"go.viam.com/rdk/metadata/service"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/framesystem"

	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/services/web"
	"go.viam.com/rdk/utils"
)

var (
	_ = robot.LocalRobot(&localRobot{})

	// defaultSvc is a list of default robot services.
	defaultSvc = []resource.Name{sensors.Name, status.Name, web.Name}
)

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	mu      sync.Mutex
	manager *resourceManager
	config  *config.Config
	logger  golog.Logger
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *localRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.manager.RemoteByName(name)
}

// ResourceByName returns a resource by name. If it does not exist
// nil is returned.
func (r *localRobot) ResourceByName(name resource.Name) (interface{}, error) {
	return r.manager.ResourceByName(name)
}

// RemoteNames returns the name of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.manager.RemoteNames()
}

// FunctionNames returns the name of all known functions.
func (r *localRobot) FunctionNames() []string {
	return r.manager.FunctionNames()
}

// ResourceNames returns the name of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.manager.ResourceNames()
}

// ProcessManager returns the process manager for the robot.
func (r *localRobot) ProcessManager() pexec.ProcessManager {
	return r.manager.processManager
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (r *localRobot) Close(ctx context.Context) error {
	return r.manager.Close(ctx)
}

// Config returns the config used to construct the robot.
// This is allowed to be partial or empty.
func (r *localRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgCpy := *r.config
	cfgCpy.Components = append([]config.Component{}, cfgCpy.Components...)

	for remoteName, remote := range r.manager.remotes {
		rc, err := remote.Config(ctx)
		if err != nil {
			return nil, err
		}
		remoteWorldName := remoteName + "." + referenceframe.World
		for _, c := range rc.Components {
			if c.Frame != nil && c.Frame.Parent == referenceframe.World {
				c.Frame.Parent = remoteWorldName
			}
			cfgCpy.Components = append(cfgCpy.Components, c)
		}
	}
	return &cfgCpy, nil
}

// getRemoteConfig gets the parameters for the Remote.
func (r *localRobot) getRemoteConfig(remoteName string) (*config.Remote, error) {
	for _, rConf := range r.config.Remotes {
		if rConf.Name == remoteName {
			return &rConf, nil
		}
	}
	return nil, fmt.Errorf("cannot find Remote config with name %q", remoteName)
}

// FrameSystem returns the FrameSystem of the robot.
func (r *localRobot) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	logger := r.Logger()
	// create the base reference frame system
	fsService, err := framesystem.FromRobot(r)
	if err != nil {
		return nil, err
	}
	parts, err := fsService.Config(ctx)
	if err != nil {
		return nil, err
	}
	baseFrameSys, err := framesystem.NewFrameSystemFromParts(name, "", parts, logger)
	if err != nil {
		return nil, err
	}
	logger.Debugf("base frame system %q has frames %v", baseFrameSys.Name(), baseFrameSys.FrameNames())
	// get frame system for each of its remote parts and merge to base
	for remoteName, remote := range r.manager.remotes {
		remoteFrameSys, err := remote.FrameSystem(ctx, remoteName, prefix)
		if err != nil {
			return nil, err
		}
		rConf, err := r.getRemoteConfig(remoteName)
		if err != nil {
			return nil, err
		}
		logger.Debugf("merging remote frame system  %q with frames %v", remoteFrameSys.Name(), remoteFrameSys.FrameNames())
		err = config.MergeFrameSystems(baseFrameSys, remoteFrameSys, rConf.Frame)
		if err != nil {
			return nil, err
		}
	}
	logger.Debugf("final frame system  %q has frames %v", baseFrameSys.Name(), baseFrameSys.FrameNames())
	return baseFrameSys, nil
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() golog.Logger {
	return r.logger
}

// New returns a new robot with parts sourced from the given config.
func New(ctx context.Context, cfg *config.Config, logger golog.Logger) (robot.LocalRobot, error) {
	r := &localRobot{
		manager: newResourceManager(
			resourceManagerOptions{
				debug:              cfg.Debug,
				fromCommand:        cfg.FromCommand,
				allowInsecureCreds: cfg.AllowInsecureCreds,
				tlsConfig:          cfg.Network.TLSConfig,
			},
			logger,
		),
		logger: logger,
	}

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.Errorw("failed to close robot down after startup failure", "error", err)
			}
		}
	}()
	r.config = cfg

	if err := r.manager.processConfig(ctx, cfg, r, logger); err != nil {
		return nil, err
	}

	// default services
	for _, name := range defaultSvc {
		cfg := config.Service{Type: config.ServiceType(name.ResourceSubtype)}
		svc, err := r.newService(ctx, cfg)
		if err != nil {
			return nil, err
		}
		r.manager.addResource(name, svc)
	}

	// update default services - done here so that all resources have been created and can be addressed.
	if err := r.updateDefaultServices(ctx); err != nil {
		return nil, err
	}

	// if metadata exists, update it
	if svc := service.ContextService(ctx); svc != nil {
		if err := r.UpdateMetadata(svc); err != nil {
			return nil, err
		}
	}
	successful = true
	return r, nil
}

func (r *localRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ServiceLookup(rName.Subtype)
	if f == nil {
		return nil, errors.Errorf("unknown service type: %s", rName.Subtype)
	}
	return f.Constructor(ctx, r, config, r.logger)
}

func (r *localRobot) newResource(ctx context.Context, config config.Component) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component subtype: %s and/or model: %s", rName.Subtype, config.Model)
	}
	newResource, err := f.Constructor(ctx, r, config, r.logger)
	if err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return newResource, nil
	}
	return c.Reconfigurable(newResource)
}

func (r *localRobot) updateDefaultServices(ctx context.Context) error {
	// grab all resources
	resources := map[resource.Name]interface{}{}
	for _, n := range r.ResourceNames() {
		// TODO(RDK-119) if not found, could mean a name clash or a remote service
		res, err := r.ResourceByName(n)
		if err != nil {
			r.logger.Debugf("not found while grabbing all resources during default svc refresh: %w", err)
		}
		resources[n] = res
	}
	for _, name := range defaultSvc {
		svc, err := r.ResourceByName(name)
		if err != nil {
			return utils.NewResourceNotFoundError(name)
		}
		updateable, ok := svc.(resource.Updateable)
		if ok {
			if err := updateable.Update(ctx, resources); err != nil {
				return err
			}
		}
	}
	return nil
}

// Refresh does nothing for now.
func (r *localRobot) Refresh(ctx context.Context) error {
	return nil
}

// UpdateMetadata updates metadata service using the currently registered parts of the robot.
func (r *localRobot) UpdateMetadata(svc service.Metadata) error {
	var resources []resource.Name

	metadata := resource.NameFromSubtype(service.Subtype, "")
	resources = append(resources, metadata)

	for _, name := range r.FunctionNames() {
		res := resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeFunction,
			resource.ResourceSubtypeFunction,
			name,
		)
		resources = append(resources, res)
	}
	for _, name := range r.RemoteNames() {
		res := resource.NewName(
			resource.ResourceNamespaceRDK,
			resource.ResourceTypeComponent,
			resource.ResourceSubtypeRemote,
			name,
		)
		resources = append(resources, res)
	}

	for _, n := range r.ResourceNames() {
		// skip web so it doesn't show up over grpc
		if n == web.Name {
			continue
		}
		resources = append(resources, n)
	}
	return svc.Replace(resources)
}
