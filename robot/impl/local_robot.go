// Package robotimpl defines implementations of robot.Robot and robot.LocalRobot.
//
// It also provides a remote robot implementation that is aware that the robot.Robot
// it is working with is not on the same physical system.
package robotimpl

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	otlpv1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/multierr"
	packagespb "go.viam.com/api/app/packages/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/cloud"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/ftdc/sys"
	icloud "go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/internal/otlpfile"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/jobmanager"
	"go.viam.com/rdk/robot/packages"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/utils"
)

const localConfigPartID = "local-config"

var _ = robot.LocalRobot(&localRobot{})

func init() {
	// Unfortunately Otel SDK doesn't have a way to reconfigure the resource
	// information so we need to set it here before any of the gRPC servers
	// access the global tracer provider.
	//nolint: errcheck
	trace.SetProvider(
		context.Background(),
		sdktrace.WithResource(
			otelresource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName("rdk"),
				semconv.ServiceNamespace("viam.com"),
			),
		),
	)
}

// localRobot satisfies robot.LocalRobot and defers most
// logic to its manager.
type localRobot struct {
	// TODO: replace all usage of [utils.ViamDotDir] with this configurable value.
	homeDir string

	manager       *resourceManager
	mostRecentCfg atomic.Value // config.Config

	operations              *operation.Manager
	sessionManager          session.Manager
	packageManager          packages.ManagerSyncer
	jobManager              *jobmanager.JobManager
	localPackages           packages.ManagerSyncer
	cloudConnSvc            icloud.ConnectionService
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup

	// reconfigurationLock manages access to the resource graph and nodes. If either may change, this lock should be taken.
	reconfigurationLock sync.Mutex
	// reconfigureWorkers tracks goroutines spawned by reconfiguration functions. we only
	// wait on this group in tests to prevent goleak-related failures. however, we do not
	// wait on this group outside of testing, since the related goroutines may be running
	// outside code and have unexpected behavior.
	reconfigureWorkers         sync.WaitGroup
	cancelBackgroundWorkers    func()
	closeContext               context.Context
	triggerConfig              chan struct{}
	configTicker               *time.Ticker
	revealSensitiveConfigDiffs bool
	shutdownCallback           func()

	// lastWeakAndOptionalDependentsRound stores the value of the resource graph's
	// logical clock when updateWeakAndOptionalDependents was called.
	lastWeakAndOptionalDependentsRound atomic.Int64

	// configRevision stores the revision of the latest config ingested during
	// reconfigurations along with a timestamp.
	configRevision   config.Revision
	configRevisionMu sync.RWMutex

	// internal services that are in the graph but we also hold onto
	webSvc   web.Service
	frameSvc framesystem.Service

	// map keyed by Module.Name. This is necessary to get the package manager to use a new folder
	// when a local tarball is updated.
	localModuleVersions map[string]semver.Version
	startFtdcOnce       sync.Once
	ftdc                *ftdc.FTDC

	// whether the robot is actively reconfiguring
	reconfiguring atomic.Bool

	// whether the robot is still initializing. this value controls what state will be
	// returned by the MachineStatus endpoint (initializing if true, running if false.)
	// configured based on the `Initial` value of applied `config.Config`s.
	initializing atomic.Bool

	traceClients atomic.Pointer[[]otlptrace.Client]
}

// ExportResourcesAsDot exports the resource graph as a DOT representation for
// visualization.
// DOT reference: https://graphviz.org/doc/info/lang.html
func (r *localRobot) ExportResourcesAsDot(index int) (resource.GetSnapshotInfo, error) {
	return r.manager.ExportDot(index)
}

// RemoteByName returns a remote robot by name. If it does not exist
// nil is returned.
func (r *localRobot) RemoteByName(name string) (robot.Robot, bool) {
	return r.manager.RemoteByName(name)
}

// WriteTraceMessages writes trace spans to any configured exporters.
func (r *localRobot) WriteTraceMessages(ctx context.Context, spans []*otlpv1.ResourceSpans) error {
	traceClients := r.traceClients.Load()
	if traceClients == nil {
		return nil
	}
	var err error
	for _, c := range *traceClients {
		err = stderrors.Join(err, c.UploadTraces(ctx, spans))
	}
	return err
}

// FindBySimpleNameAndAPI finds a resource by its simple name and API. This is queried
// through the resourceGetterForAPI for _all_ incoming gRPC requests related to a
// resource. A nil resource and an error is returned in the case of no resource found, or
// multiple matching remote resources found.
func (r *localRobot) FindBySimpleNameAndAPI(name string, api resource.API) (resource.Resource, error) {
	n, err := r.manager.resources.FindBySimpleNameAndAPI(name, api)
	if err != nil {
		return nil, err
	}
	res, err := n.Resource()
	if err != nil {
		return nil, resource.NewNotAvailableError(resource.NewName(api, name), err)
	}
	return res, nil
}

// ResourceByName returns a resource by name. It now re-routes all calls to
// FindBySimpleNameAndAPI. All incoming gRPC requests related to a resource go through
// FindBySimpleNameAndAPI. ResourceByName is only called internally for some dependency
// calculation and session code.
func (r *localRobot) ResourceByName(name resource.Name) (resource.Resource, error) {
	return r.FindBySimpleNameAndAPI(name.Name, name.API)
}

// RemoteNames returns the names of all known remote robots.
func (r *localRobot) RemoteNames() []string {
	return r.manager.RemoteNames()
}

// ResourceNames returns the names of all known resources.
func (r *localRobot) ResourceNames() []resource.Name {
	return r.manager.ResourceNames()
}

// ResourceRPCAPIs returns all known resource RPC APIs in use.
func (r *localRobot) ResourceRPCAPIs() []resource.RPCAPI {
	return r.manager.ResourceRPCAPIs()
}

// OperationManager returns the operation manager for the robot.
func (r *localRobot) OperationManager() *operation.Manager {
	return r.operations
}

// SessionManager returns the session manager for the robot.
func (r *localRobot) SessionManager() session.Manager {
	return r.sessionManager
}

// PackageManager returns the package manager for the robot.
func (r *localRobot) PackageManager() packages.Manager {
	return r.packageManager
}

// Close attempts to cleanly close down all constituent parts of the robot. It does not wait on reconfigureWorkers,
// as they may be running outside code and have unexpected behavior.
func (r *localRobot) Close(ctx context.Context) error {
	// we will stop and close web ourselves since modules need it to be
	// removed properly and in the right order, so grab it before its removed
	// from the graph/closed automatically.
	if r.webSvc != nil {
		// we may not have the web service if we closed prematurely
		r.webSvc.Stop()
	}
	if r.cancelBackgroundWorkers != nil {
		r.cancelBackgroundWorkers()
		r.cancelBackgroundWorkers = nil
		if r.configTicker != nil {
			r.configTicker.Stop()
		}
	}
	r.activeBackgroundWorkers.Wait()
	r.sessionManager.Close()

	var err error
	if r.cloudConnSvc != nil {
		err = multierr.Combine(err, r.cloudConnSvc.Close(ctx))
	}
	if r.manager != nil {
		r.reconfigurationLock.Lock()
		err = multierr.Combine(err, r.manager.Close(ctx))
		r.reconfigurationLock.Unlock()
	}
	if r.packageManager != nil {
		err = multierr.Combine(err, r.packageManager.Close(ctx))
	}
	if r.jobManager != nil {
		err = multierr.Combine(err, r.jobManager.Close())
	}
	if r.webSvc != nil {
		err = multierr.Combine(err, r.webSvc.Close(ctx))
	}
	if r.ftdc != nil {
		r.ftdc.StopAndJoin(ctx)
	}

	err = multierr.Combine(err, trace.Shutdown(ctx))

	return err
}

// Kill will attempt to kill any processes on the system started by the robot as quickly as possible.
// This operation is not clean and will not wait for completion.
func (r *localRobot) Kill() {
	r.manager.Kill()
}

// StopAll cancels all current and outstanding operations for the robot and stops all actuators and movement.
func (r *localRobot) StopAll(ctx context.Context, extra map[resource.Name]map[string]interface{}) error {
	// Stop all operations
	for _, op := range r.OperationManager().All() {
		op.Cancel()
	}

	// Stop all stoppable resources
	resourceErrs := make(map[string]error)
	for _, name := range r.ResourceNames() {
		res, err := r.ResourceByName(name)
		if err != nil {
			resourceErrs[name.Name] = err
			continue
		}

		if actuator, ok := res.(resource.Actuator); ok {
			if err := actuator.Stop(ctx, extra[name]); err != nil {
				resourceErrs[name.Name] = err
			}
		}
	}

	var errs error
	for k, v := range resourceErrs {
		errs = multierr.Combine(errs, errors.Errorf("failed to stop component named %s with error %v", k, v))
	}
	return errs
}

// Config returns a config representing the current state of the robot.
func (r *localRobot) Config() *config.Config {
	cfg := r.mostRecentCfg.Load().(config.Config)

	// Use resource manager to generate Modules, Remotes, Components, Processes
	// and Services.
	//
	// NOTE(benji): it would be great if the resource manager could somehow
	// generate Cloud, Packages, Network and Auth fields.
	generatedCfg := r.manager.createConfig()
	cfg.Modules = generatedCfg.Modules
	cfg.Remotes = generatedCfg.Remotes
	cfg.Components = generatedCfg.Components
	cfg.Processes = generatedCfg.Processes
	cfg.Services = generatedCfg.Services

	return &cfg
}

// Logger returns the logger the robot is using.
func (r *localRobot) Logger() logging.Logger {
	return r.logger
}

// StartWeb starts the web server, will return an error if server is already up.
func (r *localRobot) StartWeb(ctx context.Context, o weboptions.Options) (err error) {
	ret := r.webSvc.Start(ctx, o)
	r.startFtdcOnce.Do(func() {
		if r.ftdc != nil {
			r.ftdc.Start()
		}
	})
	return ret
}

// StopWeb stops the web server, will be a noop if server is not up.
func (r *localRobot) StopWeb() {
	r.webSvc.Stop()
}

// WebAddress return the web service's address.
func (r *localRobot) WebAddress() (string, error) {
	return r.webSvc.Address(), nil
}

// ModuleAddress return the module service's address.
func (r *localRobot) ModuleAddresses() (config.ParentSockAddrs, error) {
	return r.webSvc.ModuleAddresses(), nil
}

func (r *localRobot) sendTriggerConfig(caller string) {
	if r.closeContext.Err() != nil {
		return
	}

	// Attempt to trigger completeConfig goroutine execution when called,
	// but does not block if triggerConfig is full.
	select {
	case <-r.closeContext.Done():
		return
	case r.triggerConfig <- struct{}{}:
	default:
		r.Logger().CDebugw(
			r.closeContext,
			"attempted to trigger reconfiguration, but there is already one queued.",
			"caller", caller,
		)
	}
}

func (r *localRobot) updateRemotesAndRetryResourceConfigure() bool {
	r.reconfigurationLock.Lock()
	defer r.reconfigurationLock.Unlock()

	anyChanges := r.manager.updateRemotesResourceNames(r.closeContext)
	if r.manager.anyResourcesNotConfigured() {
		anyChanges = true
		r.manager.completeConfig(r.closeContext, r, false)
	}
	if anyChanges {
		r.updateWeakAndOptionalDependents(r.closeContext)
	}
	return anyChanges
}

// completeConfigWorker tries to complete the config and update weak/optional dependencies
// if any resources are not configured. It will also update the resource graph if remotes
// have changed. It executes every 5 seconds or when manually triggered. Manual triggers
// are sent when changes in remotes are detected and in testing.
func (r *localRobot) completeConfigWorker() {
	for {
		if r.closeContext.Err() != nil {
			return
		}

		var trigger string
		select {
		case <-r.closeContext.Done():
			return
		case <-r.configTicker.C:
			trigger = "ticker"
		case <-r.triggerConfig:
			trigger = "remote"
			r.logger.CDebugw(r.closeContext, "configuration attempt triggered by remote")
		}
		anyChanges := r.updateRemotesAndRetryResourceConfigure()
		if anyChanges {
			r.logger.CDebugw(r.closeContext, "configuration attempt completed with changes", "trigger", trigger)
		}
	}
}

func newWithResources(
	ctx context.Context,
	cfg *config.Config,
	resources map[resource.Name]resource.Resource,
	conn rpc.ClientConn,
	logger logging.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	var rOpts options
	var err error
	for _, opt := range opts {
		opt.apply(&rOpts)
	}

	partID := localConfigPartID
	if cfg.Cloud != nil {
		partID = cfg.Cloud.ID
	}

	var ftdcWorker *ftdc.FTDC
	if rOpts.enableFTDC {
		// CloudID is also known as the robot part id.
		//
		// RSDK-9369: We create a new FTDC worker, but do not yet start it. This is because the
		// `webSvc` gets registered with FTDC before we construct the underlying
		// `webSvc.rpcServer`. Which happens when calling `localRobot.StartWeb`. We've postponed
		// starting FTDC to when that method is called (the first time).
		//
		// As per the FTDC.Statser interface documentation, the return value of `webSvc.Stats` must
		// always have the same schema. Otherwise we risk the ftdc "schema" getting out of sync with
		// the data being written. Having `webSvc.Stats` conform to the API requirements is
		// challenging when we want to include stats from the `rpcServer`.
		//
		// RSDK-9369 can be reverted, having the FTDC worker getting started here, when we either:
		// - Relax the requirement that successive calls to `Stats` have the same schema or
		// - Guarantee that the `rpcServer` is initialized (enough) when the web service is
		//   constructed to get a valid copy of its stats object (for the schema's sake). Even if
		//   the web service has not been "started".
		ftdcDir := ftdc.DefaultDirectory(utils.ViamDotDir, partID)
		ftdcLogger := logger.Sublogger("ftdc")
		ftdcWorker = ftdc.NewWithUploader(ftdcDir, conn, partID, ftdcLogger)
		if statser, err := sys.NewSelfSysUsageStatser(); err == nil {
			ftdcWorker.Add("proc.viam-server", statser)
		}
		if statser, err := sys.NewNetUsageStatser(); err == nil {
			ftdcWorker.Add("net", statser)
		}
	}

	homeDir := utils.ViamDotDir
	if rOpts.viamHomeDir != "" {
		homeDir = rOpts.viamHomeDir
	}

	closeCtx, cancel := context.WithCancel(ctx)
	r := &localRobot{
		homeDir: homeDir,
		manager: newResourceManager(
			resourceManagerOptions{
				debug:              cfg.Debug,
				fromCommand:        cfg.FromCommand,
				allowInsecureCreds: cfg.AllowInsecureCreds,
				untrustedEnv:       cfg.UntrustedEnv,
				tlsConfig:          cfg.Network.TLSConfig,
				ftdc:               ftdcWorker,
			},
			logger,
		),
		operations:              operation.NewManager(logger),
		logger:                  logger,
		closeContext:            closeCtx,
		cancelBackgroundWorkers: cancel,
		// triggerConfig buffers 1 message so that we can queue up to 1 reconfiguration attempt
		// (as long as there is 1 queued, further messages can be safely discarded).
		triggerConfig:              make(chan struct{}, 1),
		configTicker:               nil,
		revealSensitiveConfigDiffs: rOpts.revealSensitiveConfigDiffs,
		cloudConnSvc:               icloud.NewCloudConnectionService(cfg.Cloud, conn, logger),
		shutdownCallback:           rOpts.shutdownCallback,
		localModuleVersions:        make(map[string]semver.Version),
		ftdc:                       ftdcWorker,
	}

	r.mostRecentCfg.Store(config.Config{})

	var heartbeatWindow time.Duration
	if cfg.Network.Sessions.HeartbeatWindow == 0 {
		heartbeatWindow = config.DefaultSessionHeartbeatWindow
	} else {
		heartbeatWindow = cfg.Network.Sessions.HeartbeatWindow
	}
	r.sessionManager = robot.NewSessionManager(r, heartbeatWindow)

	var successful bool
	defer func() {
		if !successful {
			if err := r.Close(context.Background()); err != nil {
				logger.CErrorw(ctx, "failed to close robot down after startup failure", "error", err)
			}
		}
	}()

	packageLogger := logger.Sublogger("package_manager")

	if cfg.Cloud != nil && cfg.Cloud.AppAddress != "" {
		r.packageManager = packages.NewDeferredPackageManager(
			ctx,
			func(ctx context.Context) (packagespb.PackageServiceClient, error) {
				_, cloudConn, err := r.cloudConnSvc.AcquireConnection(ctx)
				return packagespb.NewPackageServiceClient(cloudConn), err
			},
			cfg.Cloud,
			cfg.PackagePath,
			packageLogger,
		)
	} else {
		r.logger.CDebug(ctx, "Using no-op PackageManager when Cloud config is not available")
		r.packageManager = packages.NewNoopManager()
	}
	r.localPackages, err = packages.NewLocalManager(cfg, packageLogger)
	if err != nil {
		return nil, err
	}

	// we assume these never appear in our configs and as such will not be removed from the
	// resource graph
	r.webSvc = web.New(r, logger, rOpts.webOptions...)
	if r.ftdc != nil {
		r.ftdc.Add("web", r.webSvc.RequestCounter())
	}
	r.frameSvc, err = framesystem.New(ctx, resource.Dependencies{}, logger.Sublogger("framesystem"))
	if err != nil {
		return nil, err
	}

	// now that we're changing the resource graph, take the reconfigurationLock so
	// that other goroutines can't interleave
	r.reconfigurationLock.Lock()
	defer r.reconfigurationLock.Unlock()
	if err := r.manager.resources.AddNode(
		web.InternalServiceName,
		resource.NewConfiguredGraphNode(resource.Config{}, r.webSvc, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		framesystem.InternalServiceName,
		resource.NewConfiguredGraphNode(resource.Config{}, r.frameSvc, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		r.packageManager.Name(),
		resource.NewConfiguredGraphNode(resource.Config{}, r.packageManager, builtinModel)); err != nil {
		return nil, err
	}
	if err := r.manager.resources.AddNode(
		r.cloudConnSvc.Name(),
		resource.NewConfiguredGraphNode(resource.Config{}, r.cloudConnSvc, builtinModel)); err != nil {
		return nil, err
	}

	if err := r.webSvc.StartModule(ctx); err != nil {
		return nil, err
	}

	var cloudID string
	if cfg.Cloud != nil {
		cloudID = cfg.Cloud.ID
	}

	// Once web service is started, start module manager
	if err := r.manager.startModuleManager(
		closeCtx,
		r.webSvc.ModuleAddresses(),
		r.handleOrphanedResources,
		cfg.UntrustedEnv,
		homeDir,
		cloudID,
		logger,
		cfg.PackagePath,
		r.webSvc.ModPeerConnTracker(),
	); err != nil {
		return nil, err
	}

	if !rOpts.disableCompleteConfigWorker {
		r.activeBackgroundWorkers.Add(1)
		r.configTicker = time.NewTicker(5 * time.Second)
		// This goroutine will try to complete the config and update weak and optional
		// dependencies if any resources are not configured. It will also update the resource
		// graph when remotes changes or if manually triggered.
		goutils.ManagedGo(func() {
			r.completeConfigWorker()
		}, r.activeBackgroundWorkers.Done)
	}

	// getResource is passed in to the jobmanager to have access to the resource graph.
	getResource := func(res string) (resource.Resource, error) {
		var found bool
		var match resource.Name
		names := r.manager.AllNonCollidingResourceNames()
		for _, name := range names {
			if name.Name == res {
				if found {
					return nil, errors.Errorf("found duplicate entries for name %s: %s and %s", res, name.String(), match.String())
				}
				match = name
				found = true
			}
		}
		if !found {
			return nil, errors.Errorf("could not find the resource for name %s", res)
		}
		return r.ResourceByName(match)
	}

	jobManager, err := jobmanager.New(ctx, logger, getResource, r.webSvc.ModuleAddresses())
	if err != nil {
		r.logger.CErrorw(ctx, "Job manager failed to start", "error", err)
	}
	r.jobManager = jobManager

	r.reconfigure(ctx, cfg, false)

	for name, res := range resources {
		node := resource.NewConfiguredGraphNode(resource.Config{}, res, unknownModel)
		if err := r.manager.resources.AddNode(name, node); err != nil {
			return nil, err
		}
	}

	if len(resources) != 0 {
		r.updateWeakAndOptionalDependents(ctx)
	}

	successful = true
	return r, nil
}

// New returns a new robot with parts sourced from the given config.
func New(
	ctx context.Context,
	cfg *config.Config,
	conn rpc.ClientConn,
	logger logging.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	return newWithResources(ctx, cfg, nil, conn, logger, opts...)
}

// handleOrphanedResources is called by the module manager to handle resources
// orphaned due to module crashes. Resources passed into this function will be
// marked for rebuilding and handled by the completeConfig worker.
func (r *localRobot) handleOrphanedResources(ctx context.Context,
	rNames []resource.Name,
) {
	r.reconfigurationLock.Lock()
	defer r.reconfigurationLock.Unlock()
	// resource names passed into markRebuildResources are already closed as the module
	// crashed and thus do not need to be closed.
	r.manager.markRebuildResources(rNames)
	r.updateWeakAndOptionalDependents(ctx)
}

// getDependencies derives a collection of dependencies from a robot for a given
// component's name. We don't use the resource manager for this information since it has
// not been constructed at this point.
//
// Dependencies affect both the build order of resources and the access resources have to
// each other. There are four types of dependencies. Not all dependency types affect build
// order, but all dependency types affect the access resources have to each other. The
// following is a list of dependency types and a description of their behaviors:
//
// - Explicit required dependencies
//   - DEPRECATED in documentation and rarely used
//   - Specified with `depends_on` in resources' JSON configs
//   - Can be used with both modular and non-modular resources
//   - Impact build order (an explicit required dependency on a resource ensures that the
//     dependency will be constructed before the resource)
//   - Allow access to a `resource.Resource` or gRPC client referencing the dependency in
//     the resource's constructor and reconfigure methods
//   - Cause a reconfigure of the resource if the dependency fails to reconfigure (no
//     longer allowing access to that dependency in the passed-in dependencies)
//
// - Implicit required dependencies
//   - Specified with the first return value from config validation
//   - Can be used with both modular and non-modular resources
//   - Impact build order (an implicit required dependency on a resource ensures that the
//     dependency will be constructed before the resource)
//   - Allow access to a `resource.Resource` or gRPC client referencing the dependency in
//     the resource's constructor and reconfigure methods
//   - Cause a reconfigure of the resource if the dependency fails to reconfigure (no
//     longer allowing access to that dependency in the passed-in dependencies)
//
// - Implicit optional dependencies
//   - Specified with the second return value from config validation
//   - Can be used with both modular and non-modular resources
//   - Allow access to a `resource.Resource` or gRPC client referencing the dependency in
//     the resource's constructor and reconfigure methods IFF that dependency exists and
//     has been successfully constructed
//   - Cause a reconfigure of the resource if the dependency successfully constructs or
//     fails to reconfigure (no longer allowing access to that dependency in the passed-in
//     dependencies)
//
// - Weak dependencies
//   - Specified at the time of resource registration by a set of `resource.Matcher`s
//   - Can only be used with both non-modular resources
//   - Allow access to a `resource.Resource` or gRPC client referencing the dependency in
//     the resource's constructor and reconfigure methods IFF that dependency exists and
//     has been successfully constructed
//   - Cause a reconfigure of the resource if the dependency successfully constructs or
//     fails to reconfigure (no longer allowing access to that dependency in the passed-in
//     dependencies)
func (r *localRobot) getDependencies(
	rName resource.Name,
	gNode *resource.GraphNode,
) (resource.Dependencies, error) {
	if deps := gNode.UnresolvedDependencies(); len(deps) != 0 {
		return nil, errors.Errorf("resource has unresolved dependencies not found in machine config or connected remotes: %v", deps)
	}
	allDeps := make(resource.Dependencies)

	for _, dep := range r.manager.resources.GetAllParentsOf(rName) {
		// Specifically call ResourceByName and not directly to the manager since this
		// will only return fully configured and available resources (not marked for removal
		// and no last error).
		prefixedName, res, err := r.manager.ResourceByName(dep)
		if err != nil {
			return nil, &resource.DependencyNotReadyError{Name: dep.Name, Reason: err}
		}
		allDeps[prefixedName] = res
	}
	nodeConf := gNode.Config()
	for weakDepName, weakDepRes := range r.getWeakDependencies(rName, nodeConf.API, nodeConf.Model) {
		if _, ok := allDeps[weakDepName]; ok {
			continue
		}
		allDeps[weakDepName] = weakDepRes
	}
	for optionalDepName, optionalDepRes := range r.getOptionalDependencies(nodeConf) {
		if _, ok := allDeps[optionalDepName]; ok {
			continue
		}
		allDeps[optionalDepName] = optionalDepRes
	}

	return allDeps, nil
}

func (r *localRobot) getWeakDependencyMatchers(api resource.API, model resource.Model) []resource.Matcher {
	reg, ok := resource.LookupRegistration(api, model)
	if !ok {
		return nil
	}
	return reg.WeakDependencies
}

func (r *localRobot) getOptionalDependencies(conf resource.Config) resource.Dependencies {
	optDeps := make(resource.Dependencies)

	for _, optionalDepNameString := range conf.ImplicitOptionalDependsOn {
		matchingResourceNames := r.manager.resources.FindBySimpleName(optionalDepNameString)
		switch len(matchingResourceNames) {
		case 0:
			r.logger.Infow(
				"Optional dependency for resource does not exist; not passing to constructor or reconfigure yet",
				"dependency", optionalDepNameString,
				"resource", conf.ResourceName().String(),
			)
			continue
		case 1:
			if matchingResourceNames[0].String() == conf.ResourceName().String() {
				r.logger.Errorw("Resource cannot optionally depend on itself", "resource", conf.ResourceName().String())
				continue
			}
		default:
			r.logger.Errorw(
				"Cannot resolve optional dependency for resource due to multiple matching names",
				"resource", conf.ResourceName().String(),
				"conflicts", resource.NamesToStrings(matchingResourceNames),
			)
			continue
		}

		resolvedOptionalDepName := matchingResourceNames[0]

		// FindBySimpleName strips the prefix on the return, so set Name to the optionalDepNameString passed in
		// Pop the remote name off since callers won't be expecting it when accessing it in the resource
		// dependency map in a resource constructor.
		resolvedOptionalDepName.Name = optionalDepNameString
		resolvedOptionalDepName = resolvedOptionalDepName.PopRemote()

		optionalDep, err := r.ResourceByName(resolvedOptionalDepName)
		if err != nil {
			r.logger.Infow(
				"Optional dependency for resource is not available; not passing to constructor or reconfigure yet",
				"dependency", resolvedOptionalDepName.String(),
				"resource", conf.ResourceName().String(),
				"error", err,
			)
			continue
		}

		optDeps[resolvedOptionalDepName] = optionalDep
	}

	return optDeps
}

func (r *localRobot) getWeakDependencies(resName resource.Name, api resource.API, model resource.Model) resource.Dependencies {
	weakDepMatchers := r.getWeakDependencyMatchers(api, model)

	allNames := r.manager.AllNonCollidingResourceNames()
	deps := make(resource.Dependencies, len(allNames))
	for _, n := range allNames {
		if !(n.API.IsComponent() || n.API.IsService()) || n == resName {
			continue
		}
		res, err := r.ResourceByName(n)
		if err != nil {
			if !resource.IsDependencyNotReadyError(err) && !resource.IsNotAvailableError(err) {
				r.Logger().Debugw("error finding resource while getting weak dependencies", "resource", n, "error", err)
			}
			continue
		}
		for _, matcher := range weakDepMatchers {
			if matcher.IsMatch(res) {
				// Pop the remote name off since callers won't be expecting it when accessing it in the resource
				// dependency map in a resource constructor.
				deps[n.PopRemote()] = res
			}
		}
	}
	return deps
}

func (r *localRobot) newResource(
	ctx context.Context,
	gNode *resource.GraphNode,
	conf resource.Config,
) (res resource.Resource, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.Wrap(errors.Errorf("%v", r), "panic creating resource")
		}
	}()
	resName := conf.ResourceName()
	resInfo, ok := resource.LookupRegistration(resName.API, conf.Model)
	if !ok {
		failedModules := r.manager.moduleManager.FailedModules()
		var modules string
		if len(failedModules) > 0 {
			sort.Strings(failedModules)
			modules = fmt.Sprintf("May be in failing module: %v; ", failedModules)
		}
		return nil, errors.Errorf("unknown resource type: API %v with model %v not registered; "+
			"%sThere may be no module in config that provides this model", resName.API, conf.Model, modules)
	}

	deps, err := r.getDependencies(resName, gNode)
	if err != nil {
		return nil, err
	}

	c, ok := resource.LookupGenericAPIRegistration(resName.API)
	if ok {
		// If MaxInstance equals zero then there is not a limit on the number of resources
		if c.MaxInstance != 0 {
			if err := r.checkMaxInstance(resName.API, c.MaxInstance); err != nil {
				return nil, err
			}
		}
	}
	switch {
	case resInfo.Constructor != nil:
		res, err = resInfo.Constructor(ctx, deps, conf, gNode.Logger())
	case resInfo.DeprecatedRobotConstructor != nil:
		res, err = resInfo.DeprecatedRobotConstructor(ctx, r, conf, gNode.Logger())
	default:
		return nil, errors.Errorf("invariant: no constructor for %q", conf.API)
	}
	if err != nil {
		return nil, err
	}

	// If context has errored, even if construction succeeded we should close the resource and return the context error.
	// Use closeContext because otherwise any Close operations that rely on the context will immediately fail.
	// The deadline associated with the context passed in to this function is utils.GetResourceConfigurationTimeout.
	if ctx.Err() != nil {
		r.logger.CDebugw(ctx, "resource successfully constructed but context is done, closing constructed resource")
		return nil, multierr.Combine(ctx.Err(), res.Close(r.closeContext))
	}
	return res, nil
}

func (r *localRobot) updateWeakAndOptionalDependents(ctx context.Context) {
	// Track the current value of the resource graph's logical clock. This will later be
	// used to determine if updateWeakAndOptionalDependents should be called during
	// completeConfig.
	r.lastWeakAndOptionalDependentsRound.Store(r.manager.resources.CurrLogicalClockValue())

	allResources := map[resource.Name]resource.Resource{}
	internalResources := map[resource.Name]resource.Resource{}
	components := map[resource.Name]resource.Resource{}
	for _, n := range r.manager.AllNonCollidingResourceNames() {
		if !(n.API.IsComponent() || n.API.IsService()) {
			continue
		}
		res, err := r.ResourceByName(n)
		if err != nil {
			if !resource.IsDependencyNotReadyError(err) && !resource.IsNotAvailableError(err) {
				r.Logger().CDebugw(
					ctx,
					"error finding resource during weak/optional dependent update",
					"resource", n,
					"error", err,
				)
			}
			continue
		}
		allResources[n] = res
		switch {
		case n.API.IsComponent():
			components[n] = res
		case n.API.Type.Namespace == resource.APINamespaceRDKInternal:
			internalResources[n] = res
		}
	}

	timeout := utils.GetResourceConfigurationTimeout(r.logger)
	// NOTE(erd): this is intentionally hard coded since these services are treated specially with
	// how they request dependencies or consume the robot's config. We should make an effort to
	// formalize these as servcices that while internal, obey the reconfigure lifecycle.
	// For example, the framesystem should depend on all input enabled components while the web
	// service depends on all resources.
	// For now, we pass all resources and empty configs.
	processInternalResources := func(resName resource.Name, res resource.Resource, resChan chan struct{}) {
		ctxWithTimeout, timeoutCancel := context.WithTimeout(ctx, timeout)
		defer timeoutCancel()

		cleanup := utils.SlowLogger(
			ctx,
			"Waiting for internal resource to complete reconfiguration during weak/optional dependencies update",
			"resource", resName.String(),
			r.logger,
		)
		defer cleanup()

		r.reconfigureWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer func() {
				resChan <- struct{}{}
				r.reconfigureWorkers.Done()
			}()
			// NOTE(cheukt): when adding internal services that reconfigure, also add them to
			// the check in `localRobot.resourceHasWeakDependencies`.
			switch resName {
			case web.InternalServiceName:
				if err := res.Reconfigure(ctxWithTimeout, allResources, resource.Config{}); err != nil {
					r.Logger().CErrorw(
						ctx,
						"failed to reconfigure internal service during weak/optional dependencies update",
						"service", resName,
						"error", err,
					)
				}
			case framesystem.InternalServiceName:
				fsCfg, err := r.FrameSystemConfig(ctxWithTimeout)
				if err != nil {
					r.Logger().CErrorw(
						ctx,
						"failed to reconfigure internal service during weak/optional dependencies update",
						"service", resName,
						"error", err,
					)
					break
				}
				err = res.Reconfigure(ctxWithTimeout, components, resource.Config{ConvertedAttributes: fsCfg})
				if err != nil {
					r.Logger().CErrorw(
						ctx,
						"failed to reconfigure internal service during weak/optional dependencies update",
						"service", resName,
						"error", err,
					)
				}
			case packages.InternalServiceName, packages.DeferredServiceName, icloud.InternalServiceName:
			default:
				r.logger.CWarnw(
					ctx,
					"do not know how to reconfigure internal service during weak/optional dependencies update",
					"service", resName,
				)
			}
		})

		select {
		case <-resChan:
		case <-ctxWithTimeout.Done():
			if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
				r.logger.CWarn(ctx, utils.NewWeakOrOptionalDependenciesUpdateTimeoutError(resName.String(), r.logger))
			}
		case <-ctx.Done():
			return
		}
	}

	for resName, res := range internalResources {
		select {
		case <-ctx.Done():
			return
		default:
		}
		resChan := make(chan struct{}, 1)
		resName := resName
		res := res
		processInternalResources(resName, res, resChan)
	}

	updateResourceWeakAndOptionalDependents := func(ctx context.Context, conf resource.Config) {
		resName := conf.ResourceName()
		resNode, ok := r.manager.resources.Node(resName)
		if !ok {
			return
		}
		res, err := resNode.Resource()
		if err != nil {
			return
		}

		// Return early if resource has neither weak nor optional dependencies.
		if len(r.getWeakDependencyMatchers(conf.API, conf.Model)) == 0 &&
			len(conf.ImplicitOptionalDependsOn) == 0 {
			return
		}
		r.Logger().CDebugw(ctx, "handling weak/optional update for resource", "resource", resName)
		deps, err := r.getDependencies(resName, resNode)
		if err != nil {
			r.Logger().CErrorw(
				ctx,
				"failed to get dependencies during weak/optional dependencies update; skipping",
				"resource", resName,
				"error", err,
			)
			return
		}

		// Use the module manager to reconfigure the resource if it's a modular resource. This
		// would be a modular resource that has optional dependencies.
		isModular := r.manager.moduleManager.Provides(conf)
		if isModular {
			err = r.manager.moduleManager.ReconfigureResource(ctx, conf, modmanager.DepsToNames(deps))
		} else {
			err = res.Reconfigure(ctx, deps, conf)
		}
		if err != nil {
			if resource.IsMustRebuildError(err) {
				r.Logger().CErrorw(
					ctx,
					"non-modular resource uses weak/optional dependencies but is missing a Reconfigure method",
					"resource", resName,
				)
			} else {
				r.Logger().CErrorw(
					ctx,
					"failed to reconfigure resource during weak/optional dependencies update",
					"resource", resName,
					"error", err,
				)
			}
		}
	}

	cfg := r.Config()
	for _, conf := range append(cfg.Components, cfg.Services...) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conf := conf
		ctxWithTimeout, timeoutCancel := context.WithTimeout(ctx, timeout)
		defer timeoutCancel()

		cleanup := utils.SlowLogger(
			ctx,
			"Waiting for resource to complete reconfiguration during weak/optional dependencies update",
			"resource",
			conf.ResourceName().String(),
			r.logger,
		)
		resChan := make(chan struct{}, 1)
		r.reconfigureWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
			defer func() {
				cleanup()
				resChan <- struct{}{}
				r.reconfigureWorkers.Done()
			}()
			updateResourceWeakAndOptionalDependents(ctxWithTimeout, conf)
		})
		select {
		case <-resChan:
		case <-ctxWithTimeout.Done():
			if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
				r.logger.CWarn(ctx, utils.NewWeakOrOptionalDependenciesUpdateTimeoutError(conf.ResourceName().String(), r.logger))
			}
		case <-ctx.Done():
			return
		}
	}
}

// Config returns the info of each individual part that makes up the frame system
// The output of this function is to be sent over GRPC to the client, so the client
// can build its frame system. requests the remote components from the remote's frame system service.
func (r *localRobot) FrameSystemConfig(ctx context.Context) (*framesystem.Config, error) {
	localParts, err := r.getLocalFrameSystemParts(ctx)
	if err != nil {
		return nil, err
	}
	remoteParts, err := r.getRemoteFrameSystemParts(ctx)
	if err != nil {
		return nil, err
	}

	return &framesystem.Config{Parts: append(localParts, remoteParts...)}, nil
}

// getLocalFrameSystemParts collects and returns the physical parts of the robot that may have frame info,
// excluding remote robots and services, etc from the robot's config.Config.
func (r *localRobot) getLocalFrameSystemParts(ctx context.Context) ([]*referenceframe.FrameSystemPart, error) {
	cfg := r.Config()

	parts := make([]*referenceframe.FrameSystemPart, 0)
	for _, component := range cfg.Components {
		if component.Frame == nil { // no Frame means dont include in frame system.
			continue
		}

		if component.Name == referenceframe.World {
			return nil, errors.Errorf("cannot give frame system part the name %s", referenceframe.World)
		}
		if component.Frame.Parent == "" {
			return nil, errors.Errorf("parent field in frame config for part %q is empty", component.Name)
		}
		cfgCopy := &referenceframe.LinkConfig{
			ID:          component.Frame.ID,
			Translation: component.Frame.Translation,
			Orientation: component.Frame.Orientation,
			Geometry:    component.Frame.Geometry,
			Parent:      component.Frame.Parent,
		}
		if cfgCopy.ID == "" {
			cfgCopy.ID = component.Name
		}

		var model referenceframe.Model
		var err error
		switch component.ResourceName().API.SubtypeName {
		case arm.SubtypeName, gantry.SubtypeName, gripper.SubtypeName: // catch the case for all the ModelFramers
			model, err = r.extractModelFrameJSON(ctx, component.ResourceName())
			if resource.IsNotAvailableError(err) || resource.IsNotFoundError(err) {
				// When we have non-nil errors here, it is because the resource is not yet available.
				// In this case, we will exclude it from the FS. When it becomes available, it will be included.
				continue
			}

			if err != nil {
				// If there is an error getting kinematics unrelated to resource availability, log a
				// warning. It probably impacts correct operation of the application.
				r.logger.Warnw(
					"Error getting kinematics. Resource is added to the frame system, but modeling may not work correctly.",
					"res", component, "err", err)
			}
		default:
		}
		lif, err := cfgCopy.ParseConfig()
		if err != nil {
			return nil, err
		}

		parts = append(parts, &referenceframe.FrameSystemPart{FrameConfig: lif, ModelFrame: model})
	}
	return parts, nil
}

func (r *localRobot) getRemoteFrameSystemParts(ctx context.Context) ([]*referenceframe.FrameSystemPart, error) {
	cfg := r.Config()

	remoteNames := r.RemoteNames()
	remoteNameSet := make(map[string]struct{}, len(remoteNames))
	for _, val := range remoteNames {
		remoteNameSet[val] = struct{}{}
	}

	remoteParts := make([]*referenceframe.FrameSystemPart, 0)
	for _, remoteCfg := range cfg.Remotes {
		// remote could be in config without being available (remotes could be down or otherwise unavailable)
		if _, ok := remoteNameSet[remoteCfg.Name]; !ok {
			r.logger.CDebugf(ctx, "remote %q is not available, skipping", remoteCfg.Name)
			continue
		}
		// build the frame system part that connects remote world to base world
		if remoteCfg.Frame == nil { // skip over remote if it has no frame info
			r.logger.CDebugf(ctx, "remote %q has no frame config info, skipping", remoteCfg.Name)
			continue
		}

		remoteRobot, ok := r.RemoteByName(remoteCfg.Name)
		if !ok {
			return nil, errors.Errorf("cannot find remote robot %q", remoteCfg.Name)
		}

		remote, err := utils.AssertType[robot.RemoteRobot](remoteRobot)
		if err != nil {
			// should never happen
			return nil, err
		}
		if !remote.Connected() {
			r.logger.CDebugf(ctx, "remote %q is not connected, skipping", remoteCfg.Name)
			continue
		}

		lif, err := remoteCfg.Frame.ParseConfig()
		if err != nil {
			return nil, err
		}
		parentName := remoteCfg.Name + "_" + referenceframe.World
		lif.SetName(parentName)
		remoteParts = append(remoteParts, &referenceframe.FrameSystemPart{FrameConfig: lif})

		// get the parts from the remote itself
		remoteFsCfg, err := remote.FrameSystemConfig(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error from remote %q", remoteCfg.Name)
		}
		framesystem.PrefixRemoteParts(remoteFsCfg.Parts, remoteCfg.Prefix, parentName)
		remoteParts = append(remoteParts, remoteFsCfg.Parts...)
	}
	return remoteParts, nil
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func (r *localRobot) extractModelFrameJSON(ctx context.Context, name resource.Name) (referenceframe.Model, error) {
	part, err := r.ResourceByName(name)
	if err != nil {
		return nil, err
	}
	if k, ok := part.(framesystem.InputEnabled); ok {
		return k.Kinematics(ctx)
	}
	return nil, referenceframe.ErrNoModelInformation
}

// GetPose returns the pose of the specified component in the given destination frame.
func (r *localRobot) GetPose(
	ctx context.Context,
	componentName, destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	return r.frameSvc.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra)
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (r *localRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	supplementalTransforms []*referenceframe.LinkInFrame,
) (*referenceframe.PoseInFrame, error) {
	return r.frameSvc.TransformPose(ctx, pose, dst, supplementalTransforms)
}

// TransformPointCloud will transform the pointcloud to the desired frame in the robot's frame system.
// Do not move the robot between the generation of the initial pointcloud and the receipt
// of the transformed pointcloud because that will make the transformations inaccurate.
func (r *localRobot) TransformPointCloud(
	ctx context.Context,
	srcpc pointcloud.PointCloud,
	srcName, dstName string,
) (pointcloud.PointCloud, error) {
	return r.frameSvc.TransformPointCloud(ctx, srcpc, srcName, dstName)
}

// CurrentInputs returns a map of the current inputs for each component of a machine's frame system
// and a map of statuses indicating which of the machine's components may be actuated through input values.
func (r *localRobot) CurrentInputs(ctx context.Context) (referenceframe.FrameSystemInputs, error) {
	return r.frameSvc.CurrentInputs(ctx)
}

// RobotFromConfigPath is a helper to read and process a config given its path and then create a robot based on it.
func RobotFromConfigPath(
	ctx context.Context,
	cfgPath string,
	conn rpc.ClientConn,
	logger logging.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	cfg, err := config.Read(ctx, cfgPath, logger, nil)
	if err != nil {
		logger.CError(ctx, "cannot read config")
		return nil, err
	}
	return RobotFromConfig(ctx, cfg, conn, logger, opts...)
}

// RobotFromConfig is a helper to process a config and then create a robot based on it.
func RobotFromConfig(
	ctx context.Context,
	cfg *config.Config,
	conn rpc.ClientConn,
	logger logging.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	processedCfg, err := config.ProcessConfig(cfg)
	if err != nil {
		return nil, err
	}
	return New(ctx, processedCfg, conn, logger, opts...)
}

// RobotFromResources creates a new robot consisting of the given resources. Using RobotFromConfig is preferred
// to support more streamlined reconfiguration functionality.
func RobotFromResources(
	ctx context.Context,
	resources map[resource.Name]resource.Resource,
	logger logging.Logger,
	opts ...Option,
) (robot.LocalRobot, error) {
	return newWithResources(ctx, &config.Config{}, resources, nil, logger, opts...)
}

func (r *localRobot) GetModelsFromModules(ctx context.Context) ([]resource.ModuleModel, error) {
	return r.manager.moduleManager.AllModels(), nil
}

func dialRobotClient(
	ctx context.Context,
	config config.Remote,
	logger logging.Logger,
	dialOpts ...rpc.DialOption,
) (*client.RobotClient, error) {
	rOpts := []client.RobotClientOption{client.WithDialOptions(dialOpts...), client.WithRemoteName(config.Name)}

	if config.ConnectionCheckInterval != 0 {
		rOpts = append(rOpts, client.WithCheckConnectedEvery(config.ConnectionCheckInterval))
	}
	if config.ReconnectInterval != 0 {
		rOpts = append(rOpts, client.WithReconnectEvery(config.ReconnectInterval))
	}

	// only dial once per reconfiguration cycle, any failures will be retried on a ticker anyway
	rOpts = append(rOpts, client.WithInitialDialAttempts(1))

	robotClient, err := client.New(
		ctx,
		config.Address,
		logger.Sublogger("networking"),
		rOpts...,
	)
	if err != nil {
		return nil, err
	}
	return robotClient, nil
}

// Reconfigure will safely reconfigure a robot based on the given config. It will make
// a best effort to remove no longer in use parts, but if it fails to do so, they could
// possibly leak resources. The given config may be modified by Reconfigure.
func (r *localRobot) Reconfigure(ctx context.Context, newConfig *config.Config) {
	r.reconfigurationLock.Lock()
	defer r.reconfigurationLock.Unlock()
	r.reconfigure(ctx, newConfig, false)
}

// set Module.LocalVersion on Type=local modules. Call this before localPackages.Sync and in RestartModule.
func (r *localRobot) applyLocalModuleVersions(cfg *config.Config) {
	for i := range cfg.Modules {
		mod := &cfg.Modules[i]
		if mod.Type == config.ModuleTypeLocal {
			if ver, ok := r.localModuleVersions[mod.Name]; ok {
				mod.LocalVersion = ver.String()
			} else {
				mod.LocalVersion = semver.Version{}.String()
			}
		}
	}
}

func (r *localRobot) reconfigure(ctx context.Context, newConfig *config.Config, forceSync bool) {
	defer func() {
		// Always update the `initializing` value at the end of this function. Resources may
		// be equal or `reconfigure` may otherwise return early, but we still want to move
		// from a state of initializing to running as dictated by the config value.
		r.initializing.Store(newConfig.Initial)
	}()

	if !r.reconfigureAllowed(ctx, newConfig, true) {
		return
	}

	// If reconfigure is allowed, assume we are reconfiguring until this function
	// returns.
	r.reconfiguring.Store(true)
	defer func() {
		r.reconfiguring.Store(false)
	}()

	r.configRevisionMu.Lock()
	r.configRevision = config.Revision{
		Revision:    newConfig.Revision,
		LastUpdated: time.Now(),
	}
	r.configRevisionMu.Unlock()

	var allErrs error

	initialDiff, err := config.DiffConfigs(*r.Config(), *newConfig, r.revealSensitiveConfigDiffs)
	if err != nil {
		r.logger.CErrorw(ctx, "error diffing configs", "error", err)
		return
	}

	// Reconfigure tracing first so it is possible to use tracing to debug later
	// steps in the reconfigure code path.
	if !initialDiff.TracingEqual {
		r.reconfigureTracing(ctx, newConfig)
	}

	// Sync Packages before reconfiguring rest of robot and resolving references to any packages
	// in the config.
	// TODO(RSDK-1849): Make this non-blocking so other resources that do not require packages can run before package sync finishes.
	// TODO(RSDK-2710) this should really use Reconfigure for the package and should allow itself to check
	// if anything has changed.
	err = r.packageManager.Sync(ctx, newConfig.Packages, newConfig.Modules)
	if err != nil {
		// The returned error is rich, detailing each individual packages error. The underlying
		// `Sync` call is responsible for logging those errors in a readable way. We only need to
		// log that reconfiguration is exited. To minimize the distraction of reading a list of
		// verbose errors that was already logged.
		r.Logger().CErrorw(
			ctx,
			"reconfiguration aborted because cloud modules or packages download and/or unzip failed, "+
				"currently running modules will not be shutdown",
		)
		return
	}
	// For local tarball modules, we create synthetic versions for package management. The `localRobot` keeps track of these because
	// config reader would overwrite if we just stored it in config. Here, we copy the synthetic version from the `localRobot` into the
	// appropriate `config.Module` object inside the `cfg.Modules` slice. Thus, when a local tarball module is reloaded, the viam-server
	// can unpack it into a fresh directory rather than reusing the previous one.
	r.applyLocalModuleVersions(newConfig)
	err = r.localPackages.Sync(ctx, newConfig.Packages, newConfig.Modules)
	if err != nil {
		// Same as the above `Sync` call error handling. The returned error is rich, detailing each
		// individual packages error. The underlying `Sync` call is responsible for logging those
		// errors in a readable way.
		r.Logger().CErrorw(
			ctx,
			"reconfiguration aborted because local modules or packages sync failed, currently running modules will not be shutdown",
		)
		return
	}

	// Run the setup phase for new and modified modules in new config modules before proceeding with reconfiguration.
	mods := slices.Concat[[]config.Module](initialDiff.Added.Modules, initialDiff.Modified.Modules)
	for _, mod := range mods {
		if err := r.manager.moduleManager.FirstRun(ctx, mod); err != nil {
			r.logger.CErrorw(
				ctx,
				"reconfiguration aborted because of error executing first run, currently running modules will not be shutdown",
				"module", mod.Name,
				"error", err,
			)
			return
		}
	}

	if newConfig.Cloud != nil {
		r.Logger().CDebug(ctx, "updating cached config")
		if err := newConfig.StoreToCache(); err != nil {
			r.logger.CErrorw(ctx, "error storing the config", "error", err)
		}
	}

	// Add default services and process their dependencies. Dependencies may
	// already come from config validation so we check that here.
	seen := make(map[resource.API][]int)
	for idx, val := range newConfig.Services {
		seen[val.API] = append(seen[val.API], idx)
	}
	for _, name := range resource.DefaultServices() {
		existingConfIdxs, hasExistingConf := seen[name.API]
		svcCfgs := []resource.Config{}

		defaultSvcCfg := resource.Config{
			Name:  name.Name,
			Model: resource.DefaultServiceModel,
			API:   name.API,
		}

		overwritesBuiltin := false
		if hasExistingConf {
			for _, existingConfIdx := range existingConfIdxs {
				// Overwrite the builtin service if the configured service uses the same name.
				// Otherwise, allow both to coexist.
				if defaultSvcCfg.Name == newConfig.Services[existingConfIdx].Name {
					overwritesBuiltin = true
				}
				svcCfgs = append(svcCfgs, newConfig.Services[existingConfIdx])
			}
		}
		if !overwritesBuiltin {
			svcCfgs = append(svcCfgs, defaultSvcCfg)
		}

		for i, svcCfg := range svcCfgs {
			if svcCfg.ConvertedAttributes != nil || svcCfg.Attributes != nil {
				// previously processed
				continue
			}

			// we find dependencies through configs, so we must try to validate even a default config
			if reg, ok := resource.LookupRegistration(svcCfg.API, svcCfg.Model); ok && reg.AttributeMapConverter != nil {
				converted, err := reg.AttributeMapConverter(utils.AttributeMap{})
				if err != nil {
					allErrs = multierr.Combine(allErrs, errors.Wrapf(err, "error converting attributes for %s", svcCfg.API))
					continue
				}
				svcCfg.ConvertedAttributes = converted
				requiredDeps, optionalDeps, err := converted.Validate("")
				if err != nil {
					allErrs = multierr.Combine(allErrs, errors.Wrapf(err, "error getting default service dependencies for %s", svcCfg.API))
					continue
				}
				svcCfg.ImplicitDependsOn = requiredDeps
				svcCfg.ImplicitOptionalDependsOn = optionalDeps
			}
			// Update existing service configs, and the final config will be the default service, if not overridden
			if i < len(existingConfIdxs) {
				newConfig.Services[existingConfIdxs[i]] = svcCfg
			} else {
				newConfig.Services = append(newConfig.Services, svcCfg)
			}
		}
	}

	existingConfig := r.Config()
	r.mostRecentCfg.Store(*newConfig)

	// Now that we have the new config and all references are resolved, diff it
	// with the current generated config to see what has changed
	diff, err := config.DiffConfigs(*existingConfig, *newConfig, r.revealSensitiveConfigDiffs)
	if err != nil {
		r.logger.CErrorw(ctx, "error diffing the configs", "error", err)
		return
	}

	revision := diff.NewRevision()
	for _, res := range diff.UnmodifiedResources {
		r.manager.updateRevision(res.ResourceName(), revision)
	}

	// this check is deferred because it has to happen at the end of reconfigure.
	// UpdatingJobs depends on the resource graph being populated, which
	// happens after the "diff.ResourcesEqual" check during startup. However, we also want
	// to allow modifications to jobs to trigger Updates even if the rest of the config is
	// the same -- to cover these two cases, we always want to UpdateJobs at the end of reconfigure.
	defer func() {
		if !diff.JobsEqual && r.jobManager != nil {
			r.jobManager.UpdateJobs(diff)
		}
	}()

	if r.manager.moduleManager != nil {
		r.manager.moduleManager.ClearFailedModules()
	}

	if diff.ResourcesEqual {
		return
	}

	logVerb := "Construct"
	logNoun := "construction"
	if !r.initializing.Load() {
		logVerb = "Reconfigur"
		logNoun = "reconfiguration"
	}
	r.logger.CInfof(ctx, "%ving robot", logVerb)

	if r.revealSensitiveConfigDiffs {
		r.logger.CDebugf(ctx, "%ving with %+v", logVerb, diff)
	}

	// First we mark diff.Removed resources and their children for removal. Modular resources removed this way
	// will only have their children marked for update, since there is a chance that the resource is served by
	// a different module.
	resourcesToCloseBeforeComplete, _, resourcesToRebuild := r.manager.markRemoved(ctx, diff.Removed)

	// Second we attempt to Close resources.
	alreadyClosed := make(map[resource.Name]struct{}, len(resourcesToCloseBeforeComplete))
	for _, res := range resourcesToCloseBeforeComplete {
		allErrs = multierr.Combine(allErrs, r.manager.closeResource(ctx, res))
		// avoid a double close later
		alreadyClosed[res.Name()] = struct{}{}
	}

	// Third we mark resources from removed modules for rebuild. there is a chance that the removed module was actually renamed
	// and so the resources can be rebuilt.
	// Resource names passed into markRebuildResources are already closed as part of the second step above.
	r.manager.markRebuildResources(resourcesToRebuild)

	// Fourth we update the resource graph and stop any removed processes.
	allErrs = multierr.Combine(allErrs, r.manager.updateResources(ctx, diff))

	// Fifth we attempt to complete the config (see function for details) and
	// update weak and optional dependents.
	r.manager.completeConfig(ctx, r, forceSync)
	r.updateWeakAndOptionalDependents(ctx)

	// Finally we actually remove marked resources and Close any that are
	// still unclosed.
	if err := r.manager.removeMarkedAndClose(ctx, alreadyClosed); err != nil {
		allErrs = multierr.Combine(allErrs, err)
	}

	// If new config is not marked as initial, cleanup unused packages after all
	// old resources have been closed above. This ensures processes are shutdown
	// before any files are deleted they are using.
	//
	// If new config IS marked as initial, machine will be starting with no
	// modules but may immediately reconfigure to start modules that have
	// already been downloaded. Do not cleanup packages/module dirs in that case.
	if !newConfig.Initial {
		allErrs = multierr.Combine(allErrs, r.packageManager.Cleanup(ctx))
		allErrs = multierr.Combine(allErrs, r.localPackages.Cleanup(ctx))

		// Cleanup extra dirs from previous modules or rogue scripts.
		allErrs = multierr.Combine(allErrs, r.manager.moduleManager.CleanModuleDataDirectory())
	}

	if allErrs != nil {
		r.logger.CErrorw(ctx, fmt.Sprintf("The following errors were gathered during %v", logNoun), "errors", allErrs)
	} else {
		r.logger.CInfof(ctx, "Robot %ved", strings.ToLower(logVerb))
	}
}

func (r *localRobot) reconfigureTracing(ctx context.Context, newConfig *config.Config) {
	logger := r.logger.Sublogger("tracing")
	newTracingCfg := newConfig.Tracing
	if !newTracingCfg.IsEnabled() {
		prevExporters := trace.ClearExporters()
		for _, ex := range prevExporters {
			//nolint: errcheck
			ex.Shutdown(ctx)
		}
		r.traceClients.Store(nil)
		r.logger.Info("Disabled tracing")
		return
	}
	var exporters []sdktrace.SpanExporter
	var robotTraceClients []otlptrace.Client
	if newTracingCfg.Disk {
		func() {
			partID := "local-config"
			if newConfig.Cloud != nil {
				partID = newConfig.Cloud.ID
			}
			tracesDir := filepath.Join(r.homeDir, "trace", partID)
			if err := os.MkdirAll(tracesDir, 0o700); err != nil {
				logger.Errorw("failed to create directory to store traces", "err", err)
				return
			}
			logger.Infow("created trace storage dir", "dir", tracesDir)
			traceClient, err := otlpfile.NewClient(tracesDir, "traces")
			if err != nil {
				logger.Errorw("failed to create OLTP client", "err", err)
				return
			}
			exporter, err := otlptrace.New(
				context.Background(),
				traceClient,
			)
			if err != nil {
				logger.Errorw("failed to create trace exporter", "err", err)
				return
			}

			exporters = append(exporters, exporter)
			robotTraceClients = append(robotTraceClients, traceClient)
		}()
	}
	if endpoint := newTracingCfg.OTLPEndpoint; endpoint != "" {
		func() {
			opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(newTracingCfg.OTLPEndpoint)}
			if strings.HasPrefix(endpoint, "localhost:") {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}
			otlpClient := otlptracegrpc.NewClient(opts...)
			if err := otlpClient.Start(ctx); err != nil {
				logger.Errorw("Failed to start OTLP gRPC client while reconfiguring tracing", "err", err)
				return
			}

			exporter, err := otlptrace.New(ctx, otlpClient)
			if err != nil {
				logger.Errorw("Faild to create OTLP gRPC exporter while reconfiguring tracing", "err", err)
				return
			}
			robotTraceClients = append(robotTraceClients, otlpClient)
			exporters = append(exporters, exporter)
		}()
	}
	if newTracingCfg.Console {
		devExporter := perf.NewOtelDevelopmentExporter()
		exporters = append(exporters, devExporter)
		r.logger.Info("Tracing console logger enabled. " +
			"Note that traces printed to console will not include spans from modules. " +
			"Use disk or otlpEndpoint trace exporters if module tracing is required.")
		// Don't add the development exporter to the local robot. It is written to
		// assume that child spans are always delivered before their parents, which
		// won't be true for spans sent in from module processes.
	}

	// First remove all the exporters from the global tracer provider so spans
	// stop getting sent to them.
	for _, prevExporter := range trace.ClearExporters() {
		if err := prevExporter.Shutdown(ctx); err != nil {
			logger.Warnw("Error while shutting down old trace exporter", "err", err)
		}
	}

	// Now remove the same underlying clients from the local robot and attempt to
	// cleanly shut them down. Some clients such as the grpcotlp client may try
	// to flush any buffered spans during this. Swap in nil during this time so
	// that any incoming spans are just dropped rather than sent to exporters
	// that are shutting down, potentially producing an error. This may lead to
	// loss of tracing data during reconfiguration. We don't start and swap in
	// the new trace clients at this point because the new file exporter instance
	// may fight with the old one over the output file.
	prevTraceClients := r.traceClients.Swap(nil)
	if prevTraceClients != nil {
		for _, c := range *prevTraceClients {
			if err := c.Stop(ctx); err != nil {
				logger.Warnw("Error while stopping old otlp client during reconfiguration", "err", err)
			}
		}
	}

	// Start the new trace clients and install them on the local robot and the
	// global tracer provider. Tracing is fully functional at this point.
	if len(robotTraceClients) > 0 {
		successfulClients := lo.Filter(robotTraceClients, func(client otlptrace.Client, _ int) bool {
			if err := client.Start(ctx); err != nil {
				logger.Errorw("Error while starting new otlp client; reconfiguration will continue but tracing may not be functional", "err", err)
				return false
			}
			return true
		})
		r.traceClients.Store(&successfulClients)
	}
	trace.AddExporters(exporters...)
	prevConfig := r.Config().Tracing
	r.logger.Infow("Reconfigured tracing with exporters",
		"previousConsole", prevConfig.Console,
		"newConsole", newConfig.Tracing.Console,
		"previousDisk", prevConfig.Disk,
		"newDisk", newConfig.Tracing.Disk,
		"prevOtlpEndpoint", prevConfig.OTLPEndpoint,
		"newOtlpEndpoint", newConfig.Tracing.OTLPEndpoint,
	)
}

// checkMaxInstance checks to see if the local robot has reached the maximum number of a specific resource type that are local.
func (r *localRobot) checkMaxInstance(api resource.API, max int) error { //nolint: revive
	maxInstance := 0
	for _, n := range r.ResourceNames() {
		if n.API == api && !n.ContainsRemoteNames() {
			maxInstance++
			if maxInstance == max {
				return errors.Errorf("max instance number reached for resource type: %s", api)
			}
		}
	}
	return nil
}

var errNoCloudMetadata = errors.New("cloud metadata not available")

// CloudMetadata returns app-related information about the robot.
func (r *localRobot) CloudMetadata(ctx context.Context) (cloud.Metadata, error) {
	md := cloud.Metadata{}
	cfg := r.Config()
	if cfg == nil {
		return md, errNoCloudMetadata
	}
	cloud := cfg.Cloud
	if cloud == nil {
		return md, errNoCloudMetadata
	}
	md.PrimaryOrgID = cloud.PrimaryOrgID
	md.LocationID = cloud.LocationID
	md.MachineID = cloud.MachineID
	md.MachinePartID = cloud.ID
	return md, nil
}

// restartSingleModule constructs a single-module diff and calls updateResources with it.
func (r *localRobot) restartSingleModule(ctx context.Context, mod *config.Module, isRunning bool) error {
	if mod.Type == config.ModuleTypeLocal {
		// note: this version incrementing matters for local tarballs because we want the system to
		// unpack into a new directory rather than reusing the old one. Local tarball path matters
		// here because it is how artifacts are unpacked for remote reloading.
		// We technically only need to do this when mod.NeedsSyntheticPackage(), but we do it
		// for all local packages for test suite reasons.
		nextVer := r.localModuleVersions[mod.Name].IncPatch()
		r.localModuleVersions[mod.Name] = nextVer
		mod.LocalVersion = nextVer.String()
		r.logger.CInfof(ctx, "incremented local version of %s to %s", mod.Name, mod.LocalVersion)
		err := r.localPackages.SyncOne(ctx, *mod)
		if err != nil {
			return err
		}
	}

	diff := config.Diff{
		Left:     r.Config(),
		Right:    r.Config(),
		Added:    &config.Config{},
		Modified: &config.ModifiedConfigDiff{},
		Removed:  &config.Config{},
	}

	r.reconfigurationLock.Lock()
	defer r.reconfigurationLock.Unlock()
	// note: if !isRunning (i.e. the module is in config but it crashed), putting it in diff.Modified
	// results in a no-op; we use .Added instead.
	if isRunning {
		diff.Modified.Modules = append(diff.Modified.Modules, *mod)
	} else {
		diff.Added.Modules = append(diff.Added.Modules, *mod)
	}
	r.logger.CInfow(ctx, "restarting single module", "module_name", mod.Name, "module_id", mod.ModuleID)
	return r.manager.updateResources(ctx, &diff)
}

func (r *localRobot) RestartModule(ctx context.Context, req robot.RestartModuleRequest) error {
	cfg := r.mostRecentCfg.Load().(config.Config)
	var mod *config.Module
	if cfg.Modules != nil {
		mod = utils.FindInSlice(cfg.Modules, req.MatchesModule)
	}

	if mod == nil {
		return status.Errorf(codes.NotFound,
			"module not found with id=%s, name=%s. make sure it is configured and running on your machine",
			req.ModuleID, req.ModuleName)
	}

	activeModules := r.manager.createConfig().Modules
	isRunning := activeModules != nil && utils.FindInSlice(activeModules, req.MatchesModule) != nil
	err := r.restartSingleModule(ctx, mod, isRunning)
	if err != nil {
		r.logger.Warn("Error restarting module. ID: %v Name: %v Err: %v", req.ModuleID, req.ModuleName, err)
		return errors.Wrapf(err, "while restarting module id=%s, name=%s", req.ModuleID, req.ModuleName)
	}

	return nil
}

func (r *localRobot) Shutdown(ctx context.Context) error {
	if shutdownFunc := r.shutdownCallback; shutdownFunc != nil {
		shutdownFunc()
	} else {
		r.Logger().CErrorw(ctx, "shutdown function not defined")
	}
	return nil
}

// MachineStatus returns the current status of the robot.
func (r *localRobot) MachineStatus(ctx context.Context) (robot.MachineStatus, error) {
	var result robot.MachineStatus

	remoteMdMap := r.manager.getRemoteResourceMetadata(ctx)

	// we can safely ignore errors from `r.CloudMetadata`. If there is an error, that means
	// that this robot does not have CloudMetadata to attach to resources.
	md, _ := r.CloudMetadata(ctx) //nolint:errcheck
	for _, resourceStatus := range r.manager.resources.Status() {
		// if the resource is local, we can use the status as is and attach the cloud metadata of this robot.
		if !resourceStatus.Name.ContainsRemoteNames() && resourceStatus.Name.API != client.RemoteAPI {
			result.Resources = append(result.Resources, resource.Status{NodeStatus: resourceStatus, CloudMetadata: md})
			continue
		}

		// Otherwise, the resource is remote. If the corresponding status exists in remoteMdMap, use that.
		if rMd, ok := remoteMdMap[resourceStatus.Name]; ok {
			result.Resources = append(result.Resources, resource.Status{NodeStatus: resourceStatus, CloudMetadata: rMd})
			continue
		}

		// if the remote resource is not in remoteMdMap, there is a mismatch between remote resource nodes
		// in the resource graph and what was expected from getRemoteResourceMetadata. We should leave
		// cloud metadata blank in that case.
		result.Resources = append(result.Resources, resource.Status{NodeStatus: resourceStatus, CloudMetadata: cloud.Metadata{}})
	}
	r.configRevisionMu.RLock()
	result.Config = r.configRevision
	r.configRevisionMu.RUnlock()

	result.State = robot.StateRunning
	if r.initializing.Load() {
		result.State = robot.StateInitializing
	}

	if r.jobManager != nil {
		if n := r.jobManager.NumJobHistories.Load(); n > 0 {
			if result.JobStatuses == nil {
				result.JobStatuses = make(map[string]robot.JobStatus)
			}
			for jobName, jobHistory := range r.jobManager.JobHistories.Range {
				result.JobStatuses[jobName] = robot.JobStatus{
					RecentSuccessfulRuns: jobHistory.Successes(),
					RecentFailedRuns:     jobHistory.Failures(),
				}
			}
		}
	}

	return result, nil
}

// Version returns version information about the robot.
func (r *localRobot) Version(ctx context.Context) (robot.VersionResponse, error) {
	return robot.Version, nil
}

// reconfigureAllowed returns whether the local robot can reconfigure.
func (r *localRobot) reconfigureAllowed(ctx context.Context, cfg *config.Config, log bool) bool {
	// Hack: if we should not log (allowance of reconfiguration is being checked
	// from the `/restart_status` endpoint), then use a no-op logger. Otherwise
	// use robot's logger.
	logger := r.logger
	if !log {
		logger = logging.NewBlankLogger("")
	}

	// Reconfigure is always allowed in the absence of a MaintenanceConfig.
	if cfg.MaintenanceConfig == nil {
		return true
	}

	// Maintenance config can be configured to block reconfigure based off of a sensor reading
	// These sensors can be configured on the main robot, or a remote
	// In situations where there are conflicting sensor names the following behavior happens
	// Main robot and remote share sensor name -> main robot sensor is chosen
	// Only remote has the sensor name -> remote sensor is read
	// Multiple remotes share a senor name -> conflict error is returned and reconfigure happens
	// To specify a specific remote sensor use the name format remoteName:sensorName to specify a remote sensor
	name, err := resource.NewFromString(cfg.MaintenanceConfig.SensorName)
	if err != nil {
		logger.Warnf("sensor_name %s in maintenance config is not in a supported format", cfg.MaintenanceConfig.SensorName)
		return true
	}
	sensorComponent, err := robot.ResourceFromRobot[sensor.Sensor](r, name)
	if err != nil {
		logger.Warnf("%s, Starting reconfiguration", err.Error())
		return true
	}
	canReconfigure, err := r.checkMaintenanceSensorReadings(ctx, cfg.MaintenanceConfig.MaintenanceAllowedKey, sensorComponent)
	// The boolean return value of checkMaintenanceSensorReadings
	// (canReconfigure) is meaningful even when an error is also returned. Check
	// it first.
	if !canReconfigure {
		if err != nil {
			logger.CErrorw(ctx, "error reading maintenance sensor", "error", err)
		} else {
			logger.Info("maintenance_allowed_key found from readings on maintenance sensor. Skipping reconfiguration.")
		}
		diff, err := config.DiffConfigs(*r.Config(), *cfg, false)
		if err != nil {
			logger.CErrorw(ctx, "error diffing the configs", "error", err)
		}
		// NetworkEqual checks if Cloud/Auth/Network are equal between configs
		if diff != nil && !diff.NetworkEqual {
			logger.Info("Machine reconfiguration skipped but Cloud/Auth/Network config section contain changes and will be applied.")
		}
		return false
	}
	logger.Info("maintenance_allowed_key found from readings on maintenance sensor. Starting reconfiguration")

	return true
}

// checkMaintenanceSensorReadings ensures that errors from reading a sensor are handled properly.
func (r *localRobot) checkMaintenanceSensorReadings(ctx context.Context,
	maintenanceAllowedKey string, sensor resource.Sensor,
) (bool, error) {
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Context timeouts on this call should be handled by grpc
	readings, err := sensor.Readings(ctx, map[string]interface{}{})
	if err != nil {
		// if the sensor errors or timeouts we return false to block reconfigure
		return false, errors.Errorf("error reading maintenance sensor readings. %s", err.Error())
	}
	readingVal, ok := readings[maintenanceAllowedKey]
	if !ok {
		return false, errors.Errorf("error getting maintenance_allowed_key %s from sensor reading", maintenanceAllowedKey)
	}
	canReconfigure, ok := readingVal.(bool)
	if !ok {
		return false, errors.Errorf("maintenance_allowed_key %s is not a bool value", maintenanceAllowedKey)
	}
	return canReconfigure, nil
}

// RestartAllowed returns whether the robot can safely be restarted. The robot
// can be safely restarted if the robot is not in the middle of a reconfigure,
// and a reconfigure would be allowed.
func (r *localRobot) RestartAllowed() bool {
	ctx := context.Background()

	if !r.reconfiguring.Load() && r.reconfigureAllowed(ctx, r.Config(), false) {
		return true
	}
	return false
}

// ListTunnels returns information on available traffic tunnels.
func (r *localRobot) ListTunnels(_ context.Context) ([]config.TrafficTunnelEndpoint, error) {
	cfg := r.Config()
	if cfg != nil {
		return cfg.Network.NetworkConfigData.TrafficTunnelEndpoints, nil
	}
	return nil, nil
}

// GetResource implements resource.Provider for a localRobot by looking up a resource by name.
func (r *localRobot) GetResource(name resource.Name) (resource.Resource, error) {
	return r.ResourceByName(name)
}
