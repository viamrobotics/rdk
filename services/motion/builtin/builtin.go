// Package builtin implements a motion service.
package builtin

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/utils/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterDefaultService(
		motion.API,
		resource.DefaultServiceModel,
		resource.Registration[motion.Service, *Config]{
			Constructor: NewBuiltIn,
			WeakDependencies: []resource.Matcher{
				resource.TypeMatcher{Type: resource.APITypeComponentName},
				resource.SubtypeMatcher{Subtype: slam.SubtypeName},
				resource.SubtypeMatcher{Subtype: vision.SubtypeName},
			},
		},
	)
}

// export keys to be used with DoCommand so they can be referenced by clients.
const (
	DoPlan              = "plan"
	DoExecute           = "execute"
	DoExecuteCheckStart = "executeCheckStart"

	DoTeleopStart  = "teleop_start"
	DoTeleopMove   = "teleop_move"
	DoTeleopStop   = "teleop_stop"
	DoTeleopStatus = "teleop_status"
)

const (
	builtinOpLabel                     = "motion-service"
	maxTravelDistanceMM                = 5e6 // this is equivalent to 5km
	lookAheadDistanceMM        float64 = 5e6
	defaultSmoothIter                  = 30
	defaultAngularDegsPerSec           = 60.
	defaultLinearMPerSec               = 0.3
	defaultSlamPlanDeviationM          = 1.
	defaultGlobePlanDeviationM         = 2.6
	defaultCollisionBuffer             = 150. // mm
	defaultExecuteEpsilon              = 0.01 // rad or mm
)

// inputEnabledActuator is an actuator that interacts with the frame system.
// This allows us to figure out where the actuator currently is and then
// move it. Input units are always in meters or radians.
type inputEnabledActuator interface {
	resource.Actuator
	framesystem.InputEnabled
}

// Config describes how to configure the service; currently only used for specifying dependency on framesystem service.
type Config struct {
	LogFilePath string `json:"log_file_path"`
	NumThreads  int    `json:"num_threads"`

	PlanFilePath                string `json:"plan_file_path"`
	PlanDirectoryIncludeTraceID bool   `json:"plan_directory_include_trace_id"`
	LogPlannerErrors            bool   `json:"log_planner_errors"`
	LogSlowPlanThresholdMS      int    `json:"log_slow_plan_threshold_ms"`

	// example { "arm" : { "3" : { "min" : 0, "max" : 2 } } }
	InputRangeOverride map[string]map[string]referenceframe.Limit `json:"input_range_override"`

	// WorldStateStoreServiceName names a world state store service whose transforms are automatically
	// loaded as world state for Move requests. When a Move request also supplies its own WorldState,
	// the two are merged (see mergeWorldStates); the store's transforms are not ignored.
	WorldStateStoreServiceName string `json:"world_state_store_service_name"`

	// TeleopInterpolateOverride controls the arm's built-in interpolation for teleop moves.
	// When false (the default) each batch of joint targets is sent with interpolate=false and
	// waitAtEnd=false, giving the lowest latency and the most responsive motion. When true,
	// targets are sent with interpolate=true and waitAtEnd=true, which produces smoother motion
	// at the cost of latency because each call blocks until the move completes. Set it when you
	// want the arm's own interpolation in place of the pipeline's EMA smoothing.
	TeleopInterpolateOverride bool `json:"teleop_interpolate_override"`
	// TeleopSmoothAlpha is the smoothing factor for the exponential moving average (EMA) the
	// pipeline applies to each joint's commanded position before sending it to the arm:
	// out = alpha*target + (1-alpha)*previous. Lower alpha means heavier smoothing (less jitter,
	// more latency); higher alpha is more responsive and closer to the raw planned motion. The
	// valid range is (0, 1]: 1 disables smoothing, and 0 (the zero value) selects the default of 0.5.
	TeleopSmoothAlpha float64 `json:"teleop_smooth_alpha"`
}

func (c *Config) shouldWritePlan(start time.Time, err error) bool {
	if err != nil && c.LogPlannerErrors {
		return true
	}

	if c.LogSlowPlanThresholdMS != 0 &&
		time.Since(start) > (time.Duration(c.LogSlowPlanThresholdMS)*time.Millisecond) {
		return true
	}

	return false
}

// Validate here adds a dependency on the internal framesystem service.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.NumThreads < 0 {
		return nil, nil, fmt.Errorf("cannot configure with %d number of threads, number must be positive", c.NumThreads)
	}

	if c.LogPlannerErrors && c.PlanFilePath == "" {
		return nil, nil, fmt.Errorf("need a plan_file_path if you sent log_planner_errors to %v", c.LogPlannerErrors)
	}

	if c.LogSlowPlanThresholdMS != 0 && c.PlanFilePath == "" {
		return nil, nil, fmt.Errorf("need a plan_file_path if you sent LogSlowPlanThresholdMS to %v", c.LogSlowPlanThresholdMS)
	}

	// teleop_smooth_alpha must be in [0, 1]; 0 (the zero value) selects the default.
	if c.TeleopSmoothAlpha < 0 || c.TeleopSmoothAlpha > 1 {
		return nil, nil, fmt.Errorf("teleop_smooth_alpha must be in [0, 1] (0 selects the default), got %v", c.TeleopSmoothAlpha)
	}

	deps := []string{framesystem.InternalServiceName.String()}
	if c.WorldStateStoreServiceName != "" {
		deps = append(deps, worldstatestore.Named(c.WorldStateStoreServiceName).String())
	}
	return deps, nil, nil
}

type builtIn struct {
	resource.Named
	conf                    *Config
	mu                      sync.RWMutex
	fsService               framesystem.Service
	movementSensors         map[string]movementsensor.MovementSensor
	slamServices            map[string]slam.Service
	visionServices          map[string]vision.Service
	worldStateStore         worldstatestore.Service
	components              map[string]resource.Resource
	logger                  logging.Logger
	configuredDefaultExtras map[string]any

	// Teleop pipeline. Protected by teleopMu (separate from mu to simplify lock ordering).
	teleopMu       sync.RWMutex
	teleopPipeline *teleopPipeline
}

// NewBuiltIn returns a new move and grab service for the given robot.
func NewBuiltIn(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (motion.Service, error) {
	ms := &builtIn{
		Named:                   conf.ResourceName().AsNamed(),
		logger:                  logger,
		configuredDefaultExtras: make(map[string]any),
	}

	if err := ms.BuiltInReconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return ms, nil
}

// Reconfigure updates the motion service when the config has changed.
func (ms *builtIn) BuiltInReconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	// Stop teleop pipeline before acquiring write lock (goroutines may hold RLock).
	ms.teleopMu.Lock()
	if ms.teleopPipeline != nil {
		ms.teleopPipeline.stop(ctx, ms)
		ms.teleopPipeline = nil
	}
	ms.teleopMu.Unlock()

	ms.mu.Lock()
	defer ms.mu.Unlock()
	config, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	ms.conf = config

	if config.LogFilePath != "" {
		fileAppender, _ := logging.NewFileAppender(config.LogFilePath)
		ms.logger.AddAppender(fileAppender)
	}
	if config.NumThreads > 0 {
		ms.configuredDefaultExtras["num_threads"] = config.NumThreads
	}

	movementSensors := make(map[string]movementsensor.MovementSensor)
	slamServices := make(map[string]slam.Service)
	visionServices := make(map[string]vision.Service)
	worldStateStores := make(map[string]worldstatestore.Service)
	componentMap := make(map[string]resource.Resource)
	for name, dep := range deps {
		switch dep := dep.(type) {
		case framesystem.Service:
			ms.fsService = dep
		case movementsensor.MovementSensor:
			movementSensors[name.Name] = dep
		case slam.Service:
			slamServices[name.Name] = dep
		case vision.Service:
			visionServices[name.Name] = dep
		case worldstatestore.Service:
			worldStateStores[name.Name] = dep
		default:
			componentMap[name.Name] = dep
		}
	}
	ms.movementSensors = movementSensors
	ms.slamServices = slamServices
	ms.visionServices = visionServices
	ms.components = componentMap

	// Resolve the optionally-configured world state store.
	ms.worldStateStore = nil
	if name := config.WorldStateStoreServiceName; name != "" {
		store, ok := worldStateStores[name]
		if !ok {
			return errors.Errorf("configured world_state_store_service_name %q not found in dependencies", name)
		}
		ms.worldStateStore = store
	}

	return nil
}

func (ms *builtIn) Close(ctx context.Context) error {
	ms.teleopMu.Lock()
	if ms.teleopPipeline != nil {
		ms.teleopPipeline.stop(ctx, ms)
		ms.teleopPipeline = nil
	}
	ms.teleopMu.Unlock()

	return nil
}

func (ms *builtIn) Move(ctx context.Context, req motion.MoveReq) (bool, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	operation.CancelOtherWithLabel(ctx, builtinOpLabel)

	ms.applyDefaultExtras(req.Extra)
	plan, err := ms.plan(ctx, req, ms.logger)
	if err != nil {
		return false, err
	}
	err = ms.execute(ctx, plan.Trajectory(), math.MaxFloat64)
	return err == nil, err
}

func (ms *builtIn) MoveOnMap(ctx context.Context, req motion.MoveOnMapReq) (motion.ExecutionID, error) {
	return uuid.Nil, fmt.Errorf("MoveOnMap not supported by builtin")
}

func (ms *builtIn) MoveOnGlobe(ctx context.Context, req motion.MoveOnGlobeReq) (motion.ExecutionID, error) {
	return uuid.Nil, fmt.Errorf("MoveOnGlobeReqe not supported by builtin")
}

// GetPose is deprecated.
func (ms *builtIn) GetPose(
	ctx context.Context,
	componentName string,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	ms.logger.Warn("GetPose is deprecated. Please switch to using the GetPose method defined on the FrameSystem service")
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	return ms.fsService.GetPose(ctx, componentName, destinationFrame, supplementalTransforms, extra)
}

func (ms *builtIn) StopPlan(
	ctx context.Context,
	req motion.StopPlanReq,
) error {
	return fmt.Errorf("StopPlan not supported by builtin")
}

func (ms *builtIn) ListPlanStatuses(
	ctx context.Context,
	req motion.ListPlanStatusesReq,
) ([]motion.PlanStatusWithID, error) {
	return nil, fmt.Errorf("ListPlanStatuses not supported by builtin")
}

func (ms *builtIn) PlanHistory(
	ctx context.Context,
	req motion.PlanHistoryReq,
) ([]motion.PlanWithStatus, error) {
	return nil, fmt.Errorf("PlanHistory not supported by builtin")
}

// DoCommand supports two commands which are specified through the command map
//   - DoPlan generates and returns a Trajectory for a given motionpb.MoveRequest without executing it
//     required key: DoPlan
//     input value: a motionpb.MoveRequest which will be used to create a Trajectory
//     output value: a motionplan.Trajectory specified as a map (the mapstructure.Decode function is useful for decoding this)
//   - DoExecute takes a Trajectory and executes it
//     required key: DoExecute
//     input value: a motionplan.Trajectory
//     output value: a bool
func (ms *builtIn) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	// Handle teleop commands first (they manage their own locking).
	if resp, handled, err := ms.handleTeleopCommand(ctx, cmd); handled {
		return resp, err
	}

	ms.mu.RLock()
	defer ms.mu.RUnlock()
	resp := make(map[string]interface{}, 0)
	if req, ok := cmd[DoPlan]; ok {
		s, err := utils.AssertType[string](req)
		if err != nil {
			return nil, err
		}
		var moveReqProto pb.MoveRequest
		err = protojson.Unmarshal([]byte(s), &moveReqProto)
		if err != nil {
			return nil, err
		}
		fields := moveReqProto.Extra.AsMap()
		if extra, err := utils.AssertType[map[string]interface{}](fields["fields"]); err == nil {
			v, err := structpb.NewStruct(extra)
			if err != nil {
				return nil, err
			}
			moveReqProto.Extra = v
		}
		// Special handling: we want to observe the logs just for the DoCommand
		obsLogger := ms.logger.Sublogger("observed-" + uuid.New().String())
		observerCore, observedLogs := observer.New(zap.LevelEnablerFunc(zapcore.InfoLevel.Enabled))
		obsLogger.AddAppender(observerCore)

		moveReq, err := motion.MoveReqFromProto(&moveReqProto)
		if err != nil {
			return nil, err
		}
		plan, err := ms.plan(ctx, moveReq, obsLogger)
		if err != nil {
			return nil, err
		}

		partialLogString := "returning partial plan up to waypoint"
		partialLogs := observedLogs.FilterMessageSnippet(partialLogString).All()
		if len(partialLogs) > 0 {
			// Extract the waypoint number from the partial log
			if len(partialLogs) == 1 {
				logMsg := partialLogs[0].Message
				// Find the waypoint number after the partial log string
				waypointStr := strings.TrimPrefix(logMsg, partialLogString)
				// Extract just the number
				waypointNum, err := strconv.Atoi(strings.Split(strings.TrimSpace(waypointStr), " ")[0])
				if err == nil {
					resp[DoPlan+"_partialwp"] = waypointNum
				} else {
					obsLogger.CWarnf(ctx, "error parsing log string: %s", logMsg)
					obsLogger.CWarn(ctx, err)
				}
			} else {
				obsLogger.CWarnf(ctx, "Unexpected number of partial logs: %d", len(partialLogs))
			}
		}

		resp[DoPlan] = plan.Trajectory()
	}
	if req, ok := cmd[DoExecute]; ok {
		var trajectory motionplan.Trajectory
		if err := mapstructure.Decode(req, &trajectory); err != nil {
			return nil, err
		}
		// if included and set to true
		epsilon := math.MaxFloat64
		if val, ok := cmd[DoExecuteCheckStart]; ok {
			// we don't actually care if the value was set.
			// just ensure we always use a non zero, non negative epsilon
			epsilon, _ = val.(float64)
			if epsilon <= 0 {
				// use default allowable error in position for an input
				epsilon = defaultExecuteEpsilon // rad OR mm
			}

			resp[DoExecuteCheckStart] = "resource at starting location"
		}
		if err := ms.execute(ctx, trajectory, epsilon); err != nil {
			return nil, err
		}
		resp[DoExecute] = true
	}
	return resp, nil
}

func (ms *builtIn) getFrameSystem(ctx context.Context, transforms []*referenceframe.LinkInFrame) (*referenceframe.FrameSystem, error) {
	frameSys, err := framesystem.NewFromServiceMustBeConnected(ctx, ms.fsService, transforms)
	if err != nil {
		return nil, err
	}

	for fName, mods := range ms.conf.InputRangeOverride {
		f := frameSys.Frame(fName)
		if f == nil {
			return nil, fmt.Errorf("frame (%s) in input_range_override doesn't exist", fName)
		}

		ms.logger.Debugf("limit override f: %v mods: %v", fName, mods, f)

		sm, ok := f.(*referenceframe.SimpleModel)
		if !ok {
			return nil, fmt.Errorf("can only override joints for SimpleModel for now, not %T", f)
		}

		// Resolve override keys: match by name first, then by stringified moveable-frame index
		resolved := make(map[string]referenceframe.Limit, len(mods))
		moveableNames := sm.MoveableFrameNames()
		for key, limit := range mods {
			matched := false
			for i, name := range moveableNames {
				if key == name || key == strconv.Itoa(i) {
					resolved[name] = limit
					matched = true
					break
				}
			}
			if !matched {
				return nil, fmt.Errorf("can't find mod (%s)", key)
			}
		}

		newModel, err := referenceframe.NewModelWithLimitOverrides(sm, resolved)
		if err != nil {
			return nil, err
		}

		err = frameSys.ReplaceFrame(newModel)
		if err != nil {
			return nil, err
		}
	}

	return frameSys, nil
}

// normalizeTransformGeometry returns a transform whose geometry has a non-nil center pose, cloning when
// needed so the store's own proto is never mutated. World state store transforms may encode an obstacle's
// position in pose_in_observer_frame and leave the geometry center unset; the geometry converter requires
// a center, so we fill in identity (position is still applied from the transform pose by the caller).
func normalizeTransformGeometry(tf *commonpb.Transform) *commonpb.Transform {
	geo := tf.GetPhysicalObject()
	if geo == nil || geo.GetCenter() != nil {
		return tf
	}
	out := proto.Clone(tf).(*commonpb.Transform)
	out.PhysicalObject.Center = &commonpb.Pose{OZ: 1}
	return out
}

// existingFrameNames returns the set of frame names currently in the robot's frame system (plus World).
// It is used to validate that store-provided obstacles and transforms reference frames that actually
// exist, since obstacles are ultimately stapled to the world frame through that frame system.
func (ms *builtIn) existingFrameNames(ctx context.Context) map[string]bool {
	names := map[string]bool{referenceframe.World: true}
	if ms.fsService == nil {
		return names
	}
	cfg, err := ms.fsService.FrameSystemConfig(ctx)
	if err != nil {
		ms.logger.CWarnf(ctx, "motion: could not read frame system to validate world state store frames: %v", err)
		return names
	}
	for _, part := range cfg.Parts {
		if part == nil || part.FrameConfig == nil {
			continue
		}
		names[part.FrameConfig.Name()] = true
	}
	return names
}

// storeWorldStateParts reads the configured world state store service and returns its obstacles and
// supplemental transforms as raw slices. It deliberately does NOT assemble them into a WorldState, so a
// caller merging them with a request-supplied WorldState can build the result exactly once instead of
// constructing a WorldState here only to decompose it again during the merge.
//
// Each stored transform with a geometry becomes an obstacle placed at its pose within its parent frame;
// geometry-less transforms become supplemental frames. Because these obstacles are ultimately stapled
// to the world frame through the robot's frame system, any entry that references a frame that frame
// system doesn't know about — an obstacle with an unknown parent frame, or a geometry-less transform
// whose name collides with an existing frame — would otherwise error out the entire Move. We instead
// log a warning and skip that entry. Read failures are likewise logged and the store ignored. Returns
// nil slices when no store is configured or nothing survives.
func (ms *builtIn) storeWorldStateParts(
	ctx context.Context,
) ([]*referenceframe.GeometriesInFrame, []*referenceframe.LinkInFrame) {
	store := ms.worldStateStore
	if store == nil {
		return nil, nil
	}

	uuids, err := store.ListUUIDs(ctx, nil)
	if err != nil {
		ms.logger.CWarnf(ctx, "motion: failed to list transforms from world state store %q, ignoring it: %v", store.Name().Name, err)
		return nil, nil
	}

	// knownFrames accumulates the frames an obstacle may be parented to: everything already in the
	// robot's frame system, plus the store transforms we accept below.
	knownFrames := ms.existingFrameNames(ctx)

	type obstacleCandidate struct {
		parent   string
		geometry spatialmath.Geometry
		uuid     string
	}
	var candidates []obstacleCandidate
	var transforms []*referenceframe.LinkInFrame

	for _, u := range uuids {
		key := string(u)
		tf, err := store.GetTransform(ctx, u, nil)
		if err != nil {
			ms.logger.CWarnf(ctx, "motion: failed to get transform %q from world state store %q, skipping: %v", key, store.Name().Name, err)
			continue
		}
		if tf == nil {
			continue
		}

		link, err := referenceframe.LinkInFrameFromTransformProtobuf(normalizeTransformGeometry(tf))
		if err != nil {
			ms.logger.CWarnf(ctx, "motion: skipping malformed transform %q from world state store %q: %v", key, store.Name().Name, err)
			continue
		}

		if geom := link.Geometry(); geom != nil {
			// Place the geometry at the transform's pose within its parent frame and give it a unique
			// label; ObstaclesInWorldFrame then moves it into the world frame via the frame system.
			placed := geom.Transform(link.Pose())
			label := key
			if label == "" {
				label = link.Name()
			}
			placed.SetLabel(label)
			candidates = append(candidates, obstacleCandidate{parent: link.Parent(), geometry: placed, uuid: label})
			continue
		}

		// Geometry-less transform: it becomes a supplemental frame, so its name must not collide with an
		// existing frame (or an earlier store transform), or NewFrameSystem would fail the whole Move.
		if knownFrames[link.Name()] {
			ms.logger.CWarnf(ctx,
				"motion: world state store %q transform %q collides with an existing frame; skipping it", store.Name().Name, link.Name())
			continue
		}
		knownFrames[link.Name()] = true
		transforms = append(transforms, link)
	}

	// Validate obstacle parents now that knownFrames includes the accepted store transforms.
	var obstacles []*referenceframe.GeometriesInFrame
	for _, c := range candidates {
		if !knownFrames[c.parent] {
			ms.logger.CWarnf(ctx,
				"motion: world state store %q obstacle %q references frame %q which is not in the robot frame system; skipping it",
				store.Name().Name, c.uuid, c.parent)
			continue
		}
		obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(c.parent, []spatialmath.Geometry{c.geometry}))
	}

	return obstacles, transforms
}

// mergeWorldState combines a request-supplied world state with the obstacles and transforms loaded from
// the configured world state store, building the resulting WorldState exactly once. When the store
// contributed nothing the request world state is returned untouched. It performs basic validation —
// flagging malformed entries, duplicate names within a source, and name collisions introduced by the
// merge — logging a warning for each rather than dropping anything or failing the call. Colliding names
// are disambiguated (kept, not dropped) so the merged world state stays usable downstream.
//
// TODO (motion-planning-team review): follow-ups to consider — a bulk ListTransforms RPC on the store to
// replace ListUUIDs + N*GetTransform (go.viam.com/api); applying this to other world-state-consuming
// paths (e.g. MoveOnMap); subscribing to the store and caching rather than reading fresh on every Move.
func (ms *builtIn) mergeWorldState(
	ctx context.Context,
	request *referenceframe.WorldState,
	storeObstacles []*referenceframe.GeometriesInFrame,
	storeTransforms []*referenceframe.LinkInFrame,
) *referenceframe.WorldState {
	if len(storeObstacles) == 0 && len(storeTransforms) == 0 {
		return request // nothing from the store; leave the request world state untouched
	}

	// newUniqueNamer returns a validator/deduplicator for one name namespace (obstacles or transforms).
	// It logs a warning describing each duplicate or merge collision and returns a unique name so both
	// entries are kept.
	newUniqueNamer := func(kind string) func(name, source string) string {
		seen := make(map[string]string) // name -> source that first claimed it
		return func(name, source string) string {
			if name == "" {
				return name // empty obstacle labels are auto-named by NewWorldState
			}
			prev, exists := seen[name]
			if !exists {
				seen[name] = source
				return name
			}
			if prev == source {
				// Only reachable for transforms: NewWorldState already guarantees unique obstacle labels
				// within a single WorldState, so same-source obstacle duplicates can't reach here.
				ms.logger.CWarnf(ctx, "motion: duplicate %s %q within the %s world state; keeping both (renaming the duplicate)", kind, name, source)
			} else {
				ms.logger.CWarnf(ctx,
					"motion: %s %q is in both the request and the store world state; keeping both (store copy renamed)",
					kind, name)
			}
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s#%d", name, n)
				if _, taken := seen[candidate]; !taken {
					seen[candidate] = source
					return candidate
				}
			}
		}
	}

	obstacleName := newUniqueNamer("obstacle")
	obstacles := make([]*referenceframe.GeometriesInFrame, 0)
	transformName := newUniqueNamer("transform")
	transforms := make([]*referenceframe.LinkInFrame, 0)
	if request != nil {
		obstacles = ms.appendObstacles(ctx, obstacles, request.Obstacles(), "request", obstacleName)
		transforms = ms.appendTransforms(ctx, transforms, request.Transforms(), "request", transformName)
	}
	obstacles = ms.appendObstacles(ctx, obstacles, storeObstacles, "world state store", obstacleName)
	transforms = ms.appendTransforms(ctx, transforms, storeTransforms, "world state store", transformName)

	merged, err := referenceframe.NewWorldState(obstacles, transforms)
	if err != nil {
		// Names were de-duplicated above, so this is not expected; keep the request world state.
		ms.logger.CWarnf(ctx, "motion: failed to merge world states (%v); using the request world state only", err)
		return request
	}
	return merged
}

// appendObstacles validates and copies obstacles from one source into dst, de-duplicating geometry
// labels via uniqueName and defaulting a missing parent frame to World (with a warning). Geometries are
// deep-copied so the caller's world state is never mutated.
func (ms *builtIn) appendObstacles(
	ctx context.Context,
	dst, src []*referenceframe.GeometriesInFrame,
	source string,
	uniqueName func(name, source string) string,
) []*referenceframe.GeometriesInFrame {
	for _, gif := range src {
		if gif == nil {
			continue
		}
		parent := gif.Parent()
		if parent == "" {
			ms.logger.CWarnf(ctx, "motion: obstacle group in %s world state has no parent frame; assuming %q", source, referenceframe.World)
			parent = referenceframe.World
		}
		geoms := make([]spatialmath.Geometry, 0, len(gif.Geometries()))
		for _, g := range gif.Geometries() {
			if g == nil {
				ms.logger.CWarnf(ctx, "motion: dropping a nil obstacle geometry in %s world state", source)
				continue
			}
			origLabel := g.Label()
			clone := g.Transform(spatialmath.NewZeroPose()) // deep-copy so we never mutate the caller's world state
			clone.SetLabel(uniqueName(origLabel, source))
			geoms = append(geoms, clone)
		}
		if len(geoms) > 0 {
			dst = append(dst, referenceframe.NewGeometriesInFrame(parent, geoms))
		}
	}
	return dst
}

// appendTransforms validates and copies transforms from one source into dst, de-duplicating names via
// uniqueName. Renamed transforms are rebuilt so the caller's world state is never mutated.
func (ms *builtIn) appendTransforms(
	ctx context.Context,
	dst, src []*referenceframe.LinkInFrame,
	source string,
	uniqueName func(name, source string) string,
) []*referenceframe.LinkInFrame {
	for _, link := range src {
		if link == nil {
			continue
		}
		if link.Parent() == "" {
			ms.logger.CWarnf(ctx, "motion: transform %q in %s world state has no parent frame; it may not attach", link.Name(), source)
		}
		newName := uniqueName(link.Name(), source)
		if newName == link.Name() {
			dst = append(dst, link)
			continue
		}
		dst = append(dst, referenceframe.NewLinkInFrame(link.Parent(), link.Pose(), newName, link.Geometry()))
	}
	return dst
}

func (ms *builtIn) plan(ctx context.Context, req motion.MoveReq, logger logging.Logger) (motionplan.Plan, error) {
	// Merge any request-supplied world state with the one from the configured world state store so a
	// Move avoids both sets of obstacles. mergeWorldStates validates the combined set and logs warnings
	// (duplicate names, merge collisions, malformed entries) rather than dropping anything or failing.
	storeObstacles, storeTransforms := ms.storeWorldStateParts(ctx)
	req.WorldState = ms.mergeWorldState(ctx, req.WorldState, storeObstacles, storeTransforms)

	frameSys, err := ms.getFrameSystem(ctx, req.WorldState.Transforms())
	if err != nil {
		return nil, err
	}

	// build maps of relevant components and inputs from initial inputs
	fsInputs, err := ms.fsService.CurrentInputs(ctx)
	if err != nil {
		return nil, err
	}
	logger.CDebugf(ctx, "frame system inputs: %v", fsInputs)

	movingFrame := frameSys.Frame(req.ComponentName)
	if movingFrame == nil {
		return nil, fmt.Errorf("component named %s not found in robot frame system", req.ComponentName)
	}

	startState, waypoints, err := waypointsFromRequest(req, fsInputs)
	if err != nil {
		return nil, err
	}
	if len(waypoints) == 0 {
		return nil, errors.New("could not find any waypoints to plan for in MoveRequest. Fill in Destination or goal_state")
	}

	// The contents of waypoints can be gigantic, and if so, making copies of `extra` becomes the majority of motion planning runtime.
	// As the meaning from `waypoints` has already been extracted above into its proper data structure, there is no longer a need to
	// keep it in `extra`.
	if req.Extra != nil {
		req.Extra["waypoints"] = nil
	}

	// re-evaluate goal poses to be in the frame of World
	// TODO (RSDK-8847) : this is a workaround to help account for us not yet being able to properly synchronize simultaneous motion across
	// multiple components. If we are moving component1, mounted on arm2, to a goal in frame of component2, which is mounted on arm2, then
	// passing that raw poseInFrame will certainly result in a plan which moves arm1 and arm2. We cannot guarantee that this plan is
	// collision-free until RSDK-8847 is complete. By transforming goals to world, only one arm should move for such a plan.
	worldWaypoints := []*armplanning.PlanState{}
	solvingFrame := referenceframe.World
	for _, wp := range waypoints {
		if wp.Poses() != nil {
			step := referenceframe.FrameSystemPoses{}
			for fName, destination := range wp.Poses() {
				tf, err := frameSys.Transform(fsInputs.ToLinearInputs(), destination, solvingFrame)
				if err != nil {
					return nil, err
				}
				goalPose, _ := tf.(*referenceframe.PoseInFrame)
				step[fName] = goalPose
			}
			worldWaypoints = append(worldWaypoints, armplanning.NewPlanState(step, wp.Configuration()))
		} else {
			worldWaypoints = append(worldWaypoints, wp)
		}
	}

	planOpts, err := armplanning.NewPlannerOptionsFromExtra(req.Extra)
	if err != nil {
		return nil, err
	}

	// the goal is to move the component to goalPose which is specified in coordinates of goalFrameName

	obstaclesInWorldFrame, err := req.WorldState.ObstaclesInWorldFrame(frameSys, fsInputs)
	if err != nil {
		return nil, err
	}

	planRequest := &armplanning.PlanRequest{
		FrameSystem:           frameSys,
		Goals:                 worldWaypoints,
		StartState:            startState,
		ObstaclesInWorldFrame: obstaclesInWorldFrame,
		Constraints:           req.Constraints,
		PlannerOptions:        planOpts,
	}

	start := time.Now()
	plan, meta, err := armplanning.PlanMotion(ctx, logger, planRequest)
	if ms.conf.shouldWritePlan(start, err) {
		var traceID string
		if span := trace.FromContext(ctx); span != nil {
			traceID = span.SpanContext().TraceID().String()
		}

		// Extract plan tag from extra if provided
		var planTag string
		if req.Extra != nil {
			if tag, ok := req.Extra["plan_tag"].(string); ok {
				planTag = tag
			}
		}

		err := ms.writePlanRequest(planRequest, plan, meta, start, traceID, planTag, err)
		if err != nil {
			ms.logger.Warnf("couldn't write plan: %v", err)
		}
	}
	return plan, err
}

// planTeleopMulti plans a trajectory for multiple components simultaneously.
// It builds a multi-frame goal from the given poses map, allowing the planner
// to find paths for all arms jointly (self-collision between the arms is still
// enforced by the frame system's default collision checking).
//
// NOTE: unlike plan(), this path intentionally omits WorldState and Constraints
// from the PlanRequest. Teleop replans continuously at interactive rates, so it
// trades external-obstacle and custom-constraint checking for planning latency.
// If a teleop deployment needs world-obstacle avoidance, that tradeoff must be
// revisited (thread WorldState/Constraints through from the component's MoveReq).
func (ms *builtIn) planTeleopMulti(
	ctx context.Context,
	goals referenceframe.FrameSystemPoses,
	extra map[string]interface{},
	frameSys *referenceframe.FrameSystem,
	fsInputs referenceframe.FrameSystemInputs,
	logger logging.Logger,
) (motionplan.Plan, error) {
	ctx, span := trace.StartSpan(ctx, "motion::builtin::planTeleopMulti")
	defer span.End()

	// Transform all goal poses to world frame.
	worldGoals := make(referenceframe.FrameSystemPoses, len(goals))
	for fName, destination := range goals {
		tf, err := frameSys.Transform(fsInputs.ToLinearInputs(), destination, referenceframe.World)
		if err != nil {
			return nil, err
		}
		goalPose, _ := tf.(*referenceframe.PoseInFrame)
		worldGoals[fName] = goalPose
	}

	startState := armplanning.NewPlanState(nil, fsInputs)
	goalState := armplanning.NewPlanState(worldGoals, nil)

	planOpts, err := armplanning.NewPlannerOptionsFromExtra(extra)
	if err != nil {
		return nil, err
	}

	planRequest := &armplanning.PlanRequest{
		FrameSystem:    frameSys,
		Goals:          []*armplanning.PlanState{goalState},
		StartState:     startState,
		PlannerOptions: planOpts,
	}

	plan, _, err := armplanning.PlanMotion(ctx, logger, planRequest)
	return plan, err
}

func (ms *builtIn) execute(ctx context.Context, trajectory motionplan.Trajectory, epsilon float64) error {
	// Batch GoToInputs calls if possible; components may want to blend between inputs
	combinedSteps := []map[string][][]referenceframe.Input{}
	currStep := map[string][][]referenceframe.Input{}
	for i, step := range trajectory {
		if i == 0 {
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}

				r, ok := ms.components[name]
				if !ok {
					return fmt.Errorf("plan had step for resource %s but the motion service is not aware of a component of that name", name)
				}
				ie, err := utils.AssertType[framesystem.InputEnabled](r)
				if err != nil {
					return err
				}
				curr, err := ie.CurrentInputs(ctx)
				if err != nil {
					return err
				}
				if referenceframe.InputsLinfDistance(curr, inputs) > epsilon {
					return fmt.Errorf("component %v is not within %v of the current position. Expected inputs %v current inputs %v",
						name, epsilon, inputs, curr)
				}
				currStep[name] = append(currStep[name], inputs)
			}
			continue
		}
		changed := ""
		if len(currStep) > 0 {
			reset := false
			// Check if the current step moves only the same components as the previous step
			// If so, batch the inputs
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}
				if priorInputs, ok := currStep[name]; ok {
					for i, input := range inputs {
						if input != priorInputs[len(priorInputs)-1][i] {
							if changed == "" {
								changed = name
							}
							if changed != "" && changed != name {
								// If the current step moves different components than the previous step, reset the batch
								reset = true
								break
							}
						}
					}
				} else {
					// Previously moved components are no longer moving
					reset = true
				}
				if reset {
					break
				}
			}
			if reset {
				combinedSteps = append(combinedSteps, currStep)
				currStep = map[string][][]referenceframe.Input{}
			}
			for name, inputs := range step {
				if len(inputs) == 0 {
					continue
				}
				currStep[name] = append(currStep[name], inputs)
			}
		}
	}
	combinedSteps = append(combinedSteps, currStep)

	for _, step := range combinedSteps {
		for name, inputs := range step {
			if len(inputs) == 0 {
				continue
			}
			r, ok := ms.components[name]
			if !ok {
				return fmt.Errorf("plan had step for resource %s but it was not found in the motion", name)
			}
			ie, err := utils.AssertType[framesystem.InputEnabled](r)
			if err != nil {
				return err
			}
			if err := ie.GoToInputs(ctx, inputs...); err != nil {
				// If there is an error on GoToInputs, stop the component if possible before returning the error
				if actuator, ok := r.(inputEnabledActuator); ok {
					//nolint:errcheck
					_ = actuator.Stop(context.WithoutCancel(ctx), nil)
				}
				return err
			}
		}
	}
	return nil
}

// applyDefaultExtras iterates through the list of default extras configured on the builtIn motion service and adds them to the
// given map of extras if the key does not already exist.
func (ms *builtIn) applyDefaultExtras(extras map[string]any) {
	if extras == nil {
		extras = make(map[string]any)
	}
	for key, val := range ms.configuredDefaultExtras {
		if _, ok := extras[key]; !ok {
			extras[key] = val
		}
	}
}

func waypointsFromRequest(
	req motion.MoveReq,
	fsInputs referenceframe.FrameSystemInputs,
) (*armplanning.PlanState, []*armplanning.PlanState, error) {
	var startState *armplanning.PlanState
	var waypoints []*armplanning.PlanState
	var err error

	if startStateIface, ok := req.Extra["start_state"]; ok {
		if startStateMap, ok := startStateIface.(map[string]interface{}); ok {
			startState, err = armplanning.DeserializePlanState(startStateMap)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, errors.New("extras start_state could not be interpreted as map[string]interface{}")
		}
		if len(startState.Configuration()) == 0 {
			return nil, nil, fmt.Errorf("can't specify start_state without joint configuration")
		}
	} else {
		startState = armplanning.NewPlanState(nil, fsInputs)
	}

	if waypointsIface, ok := req.Extra["waypoints"]; ok {
		if waypointsIfaceList, ok := waypointsIface.([]interface{}); ok {
			for _, wpIface := range waypointsIfaceList {
				if wpMap, ok := wpIface.(map[string]interface{}); ok {
					wp, err := armplanning.DeserializePlanState(wpMap)
					if err != nil {
						return nil, nil, err
					}
					waypoints = append(waypoints, wp)
				} else {
					return nil, nil, errors.New("element in extras waypoints could not be interpreted as map[string]interface{}")
				}
			}
		} else {
			return nil, nil, errors.New("Invalid 'waypoints' extra type. Expected an array")
		}
	}

	// If goal state is specified, it overrides the request goal
	if goalStateIface, ok := req.Extra["goal_state"]; ok {
		if goalStateMap, ok := goalStateIface.(map[string]interface{}); ok {
			goalState, err := armplanning.DeserializePlanState(goalStateMap)
			if err != nil {
				return nil, nil, err
			}
			waypoints = append(waypoints, goalState)
		} else {
			return nil, nil, errors.New("extras goal_state could not be interpreted as map[string]interface{}")
		}
	} else if req.Destination != nil {
		goalState := armplanning.NewPlanState(referenceframe.FrameSystemPoses{req.ComponentName: req.Destination}, nil)
		waypoints = append(waypoints, goalState)
	}
	return startState, waypoints, nil
}

func (ms *builtIn) writePlanRequest(
	req *armplanning.PlanRequest, plan motionplan.Plan, meta *armplanning.PlanMeta, start time.Time, traceID, planTag string, planError error,
) error {
	planExtra := fmt.Sprintf("-goals-%d", len(req.Goals))
	if planError != nil {
		planExtra += "-err"
	}
	if meta.GoalsCBIRRTSolved > 0 {
		planExtra = fmt.Sprintf("%v-cbirrt%d", planExtra, meta.GoalsCBIRRTSolved)
	}

	if plan != nil {
		totalL2 := 0.0

		t := plan.Trajectory()
		for idx := 1; idx < len(t); idx++ {
			for k := range t[idx] {
				myl2n := referenceframe.InputsL2Distance(t[idx-1][k], t[idx][k])
				totalL2 += myl2n
			}
		}

		planExtra += fmt.Sprintf("-traj-%d-l2-%0.2f", len(t), totalL2)
	}

	// Add plan tag to filename if provided
	if planTag != "" {
		planExtra += fmt.Sprintf("-%s", planTag)
	}

	fn := fmt.Sprintf("plan-%s-ms-%d-%s.json",
		time.Now().Format(time.RFC3339), int(time.Since(start).Milliseconds()), planExtra)

	// Full plans (request + response) get tag=motion-plan so data manager can infer a
	// stable type tag on arbitrary-file upload.
	const motionPlanTypeTag = "motion-plan"
	tags := []string{}
	if plan != nil {
		tags = append(tags, "tag="+motionPlanTypeTag)
	}
	if ms.conf.PlanDirectoryIncludeTraceID && traceID != "" {
		tags = append(tags, "tag="+traceID)
	}
	parts := append([]string{ms.conf.PlanFilePath}, tags...)
	fn = filepath.Join(append(parts, fn)...)

	dir := filepath.Dir(fn)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	ms.logger.Infof("writing plan to %s", fn)
	return req.WriteRequestAndResponseToFile(fn, plan)
}
