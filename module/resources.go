package module

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera/rtppassthrough"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
)

// resConfigureArgs is the bookkeeping kept about a resource so it can be reconstructed
// later. `conf` is the resource's config; `depStrings` is its own dependency list, re-resolved
// into fresh handles via getDependenciesForConstruction at rebuild time.
//
// When stored in internalDeps[foo], the resConfigureArgs describes a dependent
// of foo — so internalDeps[foo][i].depStrings is the dependent's dependencies, not foo's.
type resConfigureArgs struct {
	conf       *resource.Config
	depStrings []string
}

// AddResource receives the component/service configuration from the viam-server.
func (m *Module) AddResource(ctx context.Context, req *pb.AddResourceRequest) (*pb.AddResourceResponse, error) {
	select {
	case <-m.pcReady:
	case <-m.pcFailed:
	}

	conf, err := config.ComponentConfigFromProto(req.Config, m.logger)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(conf); err != nil {
		return nil, fmt.Errorf("unable to convert attributes when adding resource: %w", err)
	}

	resInfo, ok := resource.LookupRegistration(conf.API, conf.Model)
	if !ok {
		return nil, fmt.Errorf("resource with API %q and model %q not yet registered", conf.API, conf.Model)
	}
	if resInfo.Constructor == nil {
		return nil, fmt.Errorf("invariant: no constructor for %q", conf.Model)
	}

	resLogger := m.logger.Sublogger(conf.ResourceName().String())
	levelStr := req.Config.GetLogConfiguration().GetLevel()
	// An unset LogConfiguration will materialize as an empty string.
	if levelStr != "" {
		if level, err := logging.LevelFromString(levelStr); err == nil {
			resLogger.SetLevel(level)
		} else {
			m.logger.Warnw("LogConfiguration does not contain a valid level.",
				"resource", conf.Name, "level", levelStr)
		}
	}

	err = m.addResource(ctx, req.Dependencies, conf, resLogger)
	if err != nil {
		return nil, err
	}

	return &pb.AddResourceResponse{}, nil
}

// ReconfigureResource receives the component/service configuration from the viam-server.
func (m *Module) ReconfigureResource(ctx context.Context, req *pb.ReconfigureResourceRequest) (*pb.ReconfigureResourceResponse, error) {
	// it is assumed the caller robot has handled model differences
	conf, err := config.ComponentConfigFromProto(req.Config, m.logger)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(conf); err != nil {
		return nil, fmt.Errorf("unable to convert attributes when reconfiguring resource: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.registerMu.Lock()
	deps, err := m.getDependenciesForConstruction(ctx, req.Dependencies)
	m.registerMu.Unlock()
	if err != nil {
		return nil, err
	}

	var logLevel *logging.Level
	logLevelStr := req.GetConfig().GetLogConfiguration().GetLevel()
	if level, err := logging.LevelFromString(logLevelStr); err == nil {
		// Dan: If `Reconfigure` fails, we do not undo this change. I feel it's reasonable
		// to partially reconfigure in this way.
		logLevel = &level
	} else if logLevelStr != "" {
		m.logger.Warnw("LogConfiguration does not contain a valid level",
			"resource", conf.Name, "level", logLevelStr)
	}

	if _, err = m.rebuildResource(ctx, deps, conf, logLevel); err != nil {
		return nil, err
	}

	if errors.Is(ctx.Err(), context.Canceled) {
		m.logger.Error(
			"Context was canceled before returning. Viam-server will not know the state of this resource. Module must be restarted.",
			"res", conf.Name,
		)
	}

	return &pb.ReconfigureResourceResponse{}, nil
}

// ValidateConfig receives the validation request for a resource from the viam-server.
func (m *Module) ValidateConfig(ctx context.Context,
	req *pb.ValidateConfigRequest,
) (*pb.ValidateConfigResponse, error) {
	c, err := config.ComponentConfigFromProto(req.Config, m.logger)
	if err != nil {
		return nil, err
	}

	if err := addConvertedAttributes(c); err != nil {
		return nil, fmt.Errorf("unable to convert attributes for validation: %w", err)
	}

	if c.ConvertedAttributes != nil {
		implicitRequiredDeps, implicitOptionalDeps, err := c.ConvertedAttributes.Validate(c.Name)
		if err != nil {
			return nil, fmt.Errorf("error validating resource: %w", err)
		}
		resp := &pb.ValidateConfigResponse{
			Dependencies:         implicitRequiredDeps,
			OptionalDependencies: implicitOptionalDeps,
		}
		return resp, nil
	}

	// Resource configuration object does not implement Validate, but return an
	// empty response and no error to maintain backward compatibility.
	return &pb.ValidateConfigResponse{}, nil
}

// RemoveResource receives the request for resource removal.
func (m *Module) RemoveResource(ctx context.Context, req *pb.RemoveResourceRequest) (*pb.RemoveResourceResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name, err := resource.NewFromString(req.Name)
	if err != nil {
		return nil, err
	}

	err = m.removeResource(ctx, name)
	if err != nil {
		return nil, err
	}

	return &pb.RemoveResourceResponse{}, nil
}

// GetParentResource returns a resource from the viam-server by name.
func (m *Module) GetParentResource(ctx context.Context, name resource.Name) (resource.Resource, error) {
	// Refresh parent to ensure it has the most up-to-date resources before calling
	// ResourceByName.
	if err := m.parent.Refresh(ctx); err != nil {
		return nil, err
	}
	return m.parent.ResourceByName(name)
}

// getLocalResource returns a resource from within the module by name. `getLocalResource` must be
// called while holding the `registerMu`.
func (m *Module) getLocalResource(_ context.Context, name resource.Name) (resource.Resource, error) {
	for res := range m.resLoggers {
		if res.Name() == name {
			return res, nil
		}
	}

	return nil, resource.NewNotFoundError(name)
}

// addConvertedAttributesToConfig uses the MapAttributeConverter to fill in the
// ConvertedAttributes field from the Attributes and AssociatedResourceConfigs.
func addConvertedAttributes(cfg *resource.Config) error {
	// Try to find map converter for a resource.
	reg, ok := resource.LookupRegistration(cfg.API, cfg.Model)
	if ok && reg.AttributeMapConverter != nil {
		converted, err := reg.AttributeMapConverter(cfg.Attributes)
		if err != nil {
			return fmt.Errorf("error converting attributes for resource")
		}
		cfg.ConvertedAttributes = converted
	}

	// Also try for associated configs (will only succeed if module itself registers the associated config API).
	for subIdx, associatedConf := range cfg.AssociatedResourceConfigs {
		conv, ok := resource.LookupAssociatedConfigRegistration(associatedConf.API)
		if !ok {
			continue
		}
		if conv.AttributeMapConverter != nil {
			converted, err := conv.AttributeMapConverter(associatedConf.Attributes)
			if err != nil {
				return fmt.Errorf("error converting associated resource config attributes: %w", err)
			}
			// associated resource configs for resources might be missing a resource name
			// which can be inferred from its resource config.
			converted.UpdateResourceNames(func(oldName resource.Name) resource.Name {
				return cfg.ResourceName()
			})
			cfg.AssociatedResourceConfigs[subIdx].ConvertedAttributes = converted
		}
	}
	return nil
}

// addAPIFromRegistry adds a preregistered API (rpc API) to the module's services.
func (m *Module) addAPIFromRegistry(ctx context.Context, api resource.API) error {
	m.registerMu.Lock()
	defer m.registerMu.Unlock()
	_, ok := m.collections[api]
	if ok {
		return nil
	}

	apiInfo, ok := resource.LookupGenericAPIRegistration(api)
	if !ok {
		return fmt.Errorf("invariant: registration does not exist for %q", api)
	}

	newColl := apiInfo.MakeEmptyCollection()
	m.collections[api] = newColl

	if !ok {
		return nil
	}
	return apiInfo.RegisterRPCService(ctx, m.server, newColl, m.logger)
}

// AddModelFromRegistry adds a preregistered component or service model to the module's services.
func (m *Module) AddModelFromRegistry(ctx context.Context, api resource.API, model resource.Model) error {
	resInfo, ok := resource.LookupRegistration(api, model)
	if !ok {
		return fmt.Errorf("resource with API %q and model %q not yet registered", api, model)
	}
	if resInfo.Constructor == nil {
		return fmt.Errorf("invariant: no constructor for %q", model)
	}

	m.registerMu.Lock()
	_, ok = m.collections[api]
	m.registerMu.Unlock()
	if !ok {
		if err := m.addAPIFromRegistry(ctx, api); err != nil {
			return err
		}
	}

	apiInfo, ok := resource.LookupGenericAPIRegistration(api)
	if !ok {
		return fmt.Errorf("invariant: registration does not exist for %q", api)
	}
	if apiInfo.ReflectRPCServiceDesc == nil {
		m.logger.Errorf("rpc subtype %s doesn't contain a valid ReflectRPCServiceDesc", api)
	}
	rpcAPI := resource.RPCAPI{
		API:          api,
		ProtoSvcName: apiInfo.RPCServiceDesc.ServiceName,
		Desc:         apiInfo.ReflectRPCServiceDesc,
	}

	m.registerMu.Lock()
	m.handlers[rpcAPI] = append(m.handlers[rpcAPI], model)
	m.registerMu.Unlock()
	return nil
}

// getDependenciesForConstruction must be called while holding the `registerMu`.
func (m *Module) getDependenciesForConstruction(ctx context.Context, depStrings []string,
) (resource.Dependencies, error) {
	deps := resource.Dependencies{framesystem.PublicServiceName: NewFrameSystemClient(m.parent)}
	for _, c := range depStrings {
		depName, err := resource.NewFromString(c)
		if err != nil {
			return nil, err
		}

		// If the dependency is local to this module, add the resource object directly, rather than
		// a client object that talks with the viam-server.
		localRes, err := m.getLocalResource(ctx, depName)
		if err == nil {
			deps[depName] = localRes
			continue
		}

		// Get a viam-server client object that can access the dependency.
		clientRes, err := m.GetParentResource(ctx, depName)
		if err != nil {
			return nil, err
		}
		deps[depName] = clientRes
	}

	// let modules access RobotFrameSystem (name $framesystem) without needing entire RobotClient
	return deps, nil
}

func (m *Module) addResource(
	ctx context.Context, depStrings []string, conf *resource.Config, resLogger logging.Logger,
) error {
	m.registerMu.Lock()
	deps, err := m.getDependenciesForConstruction(ctx, depStrings)
	m.registerMu.Unlock()
	if err != nil {
		return err
	}

	resInfo, ok := resource.LookupRegistration(conf.API, conf.Model)
	if !ok {
		return fmt.Errorf("resource with API %q and model %q not yet registered", conf.API, conf.Model)
	}
	if resInfo.Constructor == nil {
		return fmt.Errorf("invariant: no constructor for %q", conf.Model)
	}

	res, err := resInfo.Constructor(ctx, deps, *conf, resLogger)
	if err != nil {
		return err
	}

	// If context has errored, even if construction succeeded we should close the resource and
	// return the context error.  Use shutdownCtx because otherwise any Close operations that rely
	// on the context will immediately fail.  The deadline associated with the context passed in to
	// this function is rutils.GetResourceConfigurationTimeout, which is propagated to AddResource
	// through gRPC.
	if ctx.Err() != nil {
		m.logger.CDebugw(ctx, "resource successfully constructed but context is done, closing constructed resource",
			"err", ctx.Err().Error())
		return multierr.Combine(ctx.Err(), res.Close(m.shutdownCtx))
	}

	m.registerMu.Lock()
	defer m.registerMu.Unlock()

	coll, ok := m.collections[conf.API]
	if !ok {
		return fmt.Errorf("module cannot service api: %s", conf.API)
	}

	// If adding the resource name to the collection fails, close the resource and return an error.
	if err := coll.Add(conf.ResourceName(), res); err != nil {
		return multierr.Combine(err, res.Close(ctx))
	}

	m.resLoggers[res] = resLogger
	// add the video stream resources upon creation
	if p, ok := res.(rtppassthrough.Source); ok {
		m.streamSourceByName[res.Name()] = p
	}

	// Iterate over resource foo's dependencies and update their internalDeps entry to include newly rebuilt foo as a dependent,
	// so when we know how to reconstruct foo when it's dependencies reconstructs.
	for _, dep := range deps {
		// If the dependency is in the `resLogger` it is a "local resource". And we must track
		// rebuilds on our dependencies as that invalidates resource handles.
		//
		// Dan: We could call `m.getLocalResource(dep.Name())` but that's just a linear scan over
		// resLoggers.
		if _, exists := m.resLoggers[dep]; exists {
			depName := dep.Name()
			m.internalDeps[depName] = append(m.internalDeps[depName], resConfigureArgs{
				conf:       conf,
				depStrings: depStrings,
			})
		}
	}

	// Only cascade if (a) this resource has module-internal dependents and (b) it
	// has optional dependencies. (b) is the exact set updateWeakAndOptionalDependents rebuilds
	// for modular resources (modules can't have weak dependencies) — the only rebuild path
	// that bypasses viam-server's dependency propagation. Checking (a) first short-circuits the
	// Validate call in the no-dependents case.
	if _, hasDependents := m.internalDeps[conf.ResourceName()]; hasDependents && hasOptionalDependencies(conf) {
		m.cascadeRebuildDependentsOf(ctx, conf.ResourceName())
	}

	return nil
}

// hasOptionalDependencies returns true if `conf` declares any optional
// dependencies via its Validate method.
func hasOptionalDependencies(conf *resource.Config) bool {
	if conf.ConvertedAttributes == nil {
		return false
	}
	// NOTE: We have already used Validate to calculate optional dependencies in ValidateConfig for this
	// config, but the AddResource flow does not distinguish between optional and required dependencies
	// (it just passes an optional dependency as a normal one as long as it's available), so we must call
	// Validate again here. It would add more complexity to cache the already-calculated value, so for now
	// we are OK paying the cost of Validate again.
	_, optionalDeps, err := conf.ConvertedAttributes.Validate(conf.Name)
	if err != nil {
		// Validate failed; fall through to cascade conservatively rather than
		// silently skipping work that may be needed.
		return true
	}
	return len(optionalDeps) > 0
}

// cascadeRebuildDependentsOf rebuilds any module-internal dependents that hold a stale Go
// pointer to `newResName`. The RDK's updateWeakAndOptionalDependents flow rebuilds a
// modular resource via RemoveResource+AddResource without notifying dependents; any
// dependent that captured a direct pointer to the rebuilt resource is left stale. This
// closes that gap on the module side.
//
// Must be called with `registerMu` held; the mutex is released and reacquired around each
// recursive rebuild call.
//
// Cycle handling: we seed `visited` with newResName and thread it through the recursive
// rebuild. If a cascade would re-enter a resource already on the stack, we skip it and log
// — one side of a mutual-optional cycle keeps a stale pointer until its next reconfigure.
func (m *Module) cascadeRebuildDependentsOf(ctx context.Context, newResName resource.Name) {
	dependents, ok := m.internalDeps[newResName]
	if !ok {
		return
	}
	visited := map[resource.Name]struct{}{newResName: {}}

	for _, args := range dependents {
		if _, ok := m.collections[args.conf.API]; !ok {
			continue
		}
		freshDeps, err := m.getDependenciesForConstruction(ctx, args.depStrings)
		if err != nil {
			m.logger.Warnw("failed to get deps for cascade rebuild after resource re-add",
				"changedResource", newResName, "dependent", args.conf.Name, "err", err)
			continue
		}
		m.registerMu.Unlock()
		_, err = m.rebuildResourceWithVisited(ctx, freshDeps, args.conf, nil, visited)
		m.registerMu.Lock()
		if err != nil {
			m.logger.Warnw("failed to cascade rebuild dependent after resource re-add",
				"changedResource", newResName, "dependent", args.conf.Name, "err", err)
		}
	}
}

func (m *Module) removeResource(ctx context.Context, resName resource.Name) error {
	slowWatcher, slowWatcherCancel := utils.SlowGoroutineWatcher(
		30*time.Second, fmt.Sprintf("module resource %q is taking a while to remove", resName), m.logger)
	defer func() {
		slowWatcherCancel()
		<-slowWatcher
	}()

	m.registerMu.Lock()
	coll, ok := m.collections[resName.API]
	if !ok {
		m.registerMu.Unlock()
		return fmt.Errorf("no grpc service for %+v", resName)
	}

	res, err := coll.Resource(resName.Name)
	if err != nil {
		m.registerMu.Unlock()
		return err
	}
	m.registerMu.Unlock()

	if err := res.Close(ctx); err != nil {
		m.logger.Error(err)
	}

	m.registerMu.Lock()
	defer m.registerMu.Unlock()
	delete(m.streamSourceByName, res.Name())
	delete(m.activeResourceStreams, res.Name())
	delete(m.resLoggers, res)

	// Clear the removed resource from any dependency chain it appears in as a dependent. We do NOT
	// deliberately delete the name-keyed entry for `res` itself — if `res` is re-added later, the
	// cascade step in addResource needs that entry to find stale dependents and rebuild them. (An
	// entry that ends up empty after filtering is dropped here as routine cleanup.)
	for depName, chainReconfigures := range m.internalDeps {
		filtered := make([]resConfigureArgs, 0, len(chainReconfigures))
		for _, chainRes := range chainReconfigures {
			if chainRes.conf.ResourceName() != resName {
				filtered = append(filtered, chainRes)
			}
		}
		if len(filtered) == 0 {
			delete(m.internalDeps, depName)
		} else {
			m.internalDeps[depName] = filtered
		}
	}

	return coll.Remove(resName)
}

// rebuildResource will rebuild resource and, if successful, return the new resource
// pointer/interface object.
func (m *Module) rebuildResource(
	ctx context.Context, deps resource.Dependencies, conf *resource.Config, logLevel *logging.Level,
) (resource.Resource, error) {
	return m.rebuildResourceWithVisited(ctx, deps, conf, logLevel, map[resource.Name]struct{}{})
}

// rebuildResourceWithVisited is the recursive body of rebuildResource. It tracks which
// resources are currently on the cascade stack (in `visited`) to prevent infinite recursion
// through mutual-dependency cycles. The current resource is added to `visited` on entry and
// removed on exit (stack discipline) so that sibling branches of an outer cascade can be
// rebuilt independently.
func (m *Module) rebuildResourceWithVisited(
	ctx context.Context, deps resource.Dependencies, conf *resource.Config, logLevel *logging.Level,
	visited map[resource.Name]struct{},
) (resource.Resource, error) {
	visited[conf.ResourceName()] = struct{}{}
	defer delete(visited, conf.ResourceName())

	m.registerMu.Lock()
	coll, ok := m.collections[conf.API]
	if !ok {
		m.registerMu.Unlock()
		return nil, fmt.Errorf("no rpc service for %+v", conf)
	}

	res, err := coll.Resource(conf.ResourceName().Name)
	if err != nil {
		m.registerMu.Unlock()
		return nil, err
	}

	resLogger, hasLogger := m.resLoggers[res]
	m.registerMu.Unlock()
	if hasLogger && logLevel != nil {
		resLogger.SetLevel(*logLevel)
	}

	if err := res.Close(ctx); err != nil {
		m.logger.Error(err)
	}

	m.registerMu.Lock()
	delete(m.activeResourceStreams, res.Name())
	m.registerMu.Unlock()

	resInfo, ok := resource.LookupRegistration(conf.API, conf.Model)
	if !ok {
		return nil, fmt.Errorf("resource with API %q and model %q not yet registered", conf.API, conf.Model)
	}
	if resInfo.Constructor == nil {
		return nil, fmt.Errorf("invariant: no constructor for %q", conf.Model)
	}

	newRes, err := resInfo.Constructor(ctx, deps, *conf, m.logger)
	if err != nil {
		return nil, err
	}

	if err := coll.ReplaceOne(conf.ResourceName(), newRes); err != nil {
		return nil, multierr.Combine(err, newRes.Close(ctx))
	}

	m.registerMu.Lock()
	// We're modifying internal module maps now. We must not error out at this point without rolling
	// back module state mutations.
	delete(m.resLoggers, res)
	m.resLoggers[newRes] = resLogger

	if p, ok := newRes.(rtppassthrough.Source); ok {
		m.streamSourceByName[res.Name()] = p
	}

	resName := res.Name()
	depsToRebuild := m.internalDeps[resName]
	// Build up a new slice to map `m.internalDeps[resName]` to.
	newDepsToRebuild := make([]resConfigureArgs, 0, len(depsToRebuild))
	for _, depToReconfig := range depsToRebuild {
		if _, cycled := visited[depToReconfig.conf.ResourceName()]; cycled {
			// This dependent is already on the cascade stack. Rebuilding would close
			// a resource that an outer caller still holds. Keep the entry unchanged.
			m.logger.Warnw(
				"detected mutual-optional dependency cycle: one of these resources will always hold a closed handle to the other "+
					"after reconstruction. Remove the optional dependency from one side to break the cycle.",
				"resource", resName, "dependent", depToReconfig.conf.Name)
			newDepsToRebuild = append(newDepsToRebuild, depToReconfig)
			continue
		}
		deps, err := m.getDependenciesForConstruction(ctx, depToReconfig.depStrings)
		if err != nil {
			m.logger.Warn("Failed to get dependencies for cascading dependent reconfigure",
				"changedResource", conf.Name,
				"dependent", depToReconfig.conf.Name,
				"dependentDeps", depToReconfig.depStrings,
				"err", err)
			continue
		}

		// We release the `registerMu` to let other resource query/acquisition methods make
		// progress. We do not assume `rebuildResource` is fast.
		//
		// We also release the mutex as the recursive call to `rebuildResourceWithVisited`
		// will reacquire it. And the mutex is not reentrant.
		m.registerMu.Unlock()

		var nilLogLevel *logging.Level // pass in nil to avoid changing the log level
		if _, err := m.rebuildResourceWithVisited(ctx, deps, depToReconfig.conf, nilLogLevel, visited); err != nil {
			m.logger.Warn("Failed to cascade dependent reconfigure",
				"changedResource", conf.Name,
				"dependent", depToReconfig.conf.Name,
				"err", err)
		}
		m.registerMu.Lock()

		newDepsToRebuild = append(newDepsToRebuild, resConfigureArgs{
			conf:       depToReconfig.conf,
			depStrings: depToReconfig.depStrings,
		})
	}

	m.internalDeps[resName] = newDepsToRebuild
	m.registerMu.Unlock()

	return newRes, nil
}
