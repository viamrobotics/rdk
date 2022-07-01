package robotimpl

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/discovery"
	"go.viam.com/rdk/operation"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
)

// Reconfigure will safely reconfigure a robot based on the given config. It will make
// a best effort to remove no longer in use parts, but if it fails to do so, they could
// possibly leak resources.
func (r *localRobot) Reconfigure(ctx context.Context, newConfig *config.Config) error {
	diff, err := config.DiffConfigs(r.config, newConfig)
	if err != nil {
		return err
	}
	if diff.ResourcesEqual {
		return nil
	}
	r.logger.Debugf("reconfiguring with %+v", diff)
	draft := newDraftRobot(r, diff)
	err = draft.ProcessAndCommit(ctx)
	if err != nil {
		return err
	}

	// update default services
	r.updateDefaultServices(ctx)

	r.manager.updateRemotesResourceNames(ctx)
	return nil
}

// A draftRobot is responsible for the workflow of turning in
// a newly proposed robot into a robot ready to be swapped in
// for an existing one. It understands how to rollback and commit
// changes as safe as it possibly can.
type draftRobot struct {
	original  *localRobot
	diff      *config.Diff
	manager   *resourceManager
	leftovers PartsMergeResult
	removals  *resourceManager
}

func (draft *draftRobot) RemoteByName(name string) (robot.Robot, bool) {
	return draft.original.RemoteByName(name)
}

// ResourceByName returns a resource by name.
func (draft *draftRobot) ResourceByName(name resource.Name) (interface{}, error) {
	iface, err := draft.manager.ResourceByName(name)
	if err != nil {
		return draft.original.ResourceByName(name)
	}

	return iface, nil
}

// RemoteNames returns the name of all known remote robots.
func (draft *draftRobot) RemoteNames() []string {
	return draft.original.RemoteNames()
}

// ResourceNames returns a list of all known resource names.
func (draft *draftRobot) ResourceNames() []resource.Name {
	return draft.original.ResourceNames()
}

// ResourceRPCSubtypes returns a list of all known resource subtypes.
func (draft *draftRobot) ResourceRPCSubtypes() []resource.RPCSubtype {
	return draft.original.ResourceRPCSubtypes()
}

// ProcessManager returns the process manager for the robot.
func (draft *draftRobot) ProcessManager() pexec.ProcessManager {
	return draft.original.ProcessManager()
}

// OperationManager returns the operation manager the robot is using.
func (draft *draftRobot) OperationManager() *operation.Manager {
	return draft.original.OperationManager()
}

// Logger returns the logger the robot is using.
func (draft *draftRobot) Logger() golog.Logger {
	return draft.original.Logger()
}

// DiscoverComponents takes a list of discovery queries and returns corresponding
// component configurations.
func (draft *draftRobot) DiscoverComponents(ctx context.Context, qs []discovery.Query) ([]discovery.Discovery, error) {
	return draft.original.DiscoverComponents(ctx, qs)
}

// FrameSystemConfig returns the info of each individual part that makes
// up a robot's frame system.
func (draft *draftRobot) FrameSystemConfig(
	ctx context.Context,
	additionalTransforms []*commonpb.Transform,
) (framesystemparts.Parts, error) {
	return draft.original.FrameSystemConfig(ctx, additionalTransforms)
}

func (draft *draftRobot) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]robot.Status, error) {
	return draft.original.GetStatus(ctx, resourceNames)
}

// Close attempts to cleanly close down all constituent parts of the robot.
func (draft *draftRobot) Close(ctx context.Context) error {
	return draft.original.Close(ctx)
}

func (draft *draftRobot) newService(ctx context.Context, config config.Service) (interface{}, error) {
	return draft.original.newService(ctx, config)
}

// getDependencies derives a collection of dependencies from the resource manager of a
// draft robot for a given component configuration.
func (draft *draftRobot) getDependencies(config config.Component) (registry.Dependencies, error) {
	deps := make(registry.Dependencies)
	for _, dep := range draft.manager.resources.GetAllParentsOf(config.ResourceName()) {
		res, err := draft.manager.ResourceByName(dep)
		if err != nil {
			return nil, &registry.DependencyNotReadyError{Name: dep.Name}
		}
		deps[dep] = res
	}

	return deps, nil
}

func (draft *draftRobot) newResource(ctx context.Context, config config.Component) (interface{}, error) {
	rName := config.ResourceName()
	f := registry.ComponentLookup(rName.Subtype, config.Model)
	if f == nil {
		return nil, errors.Errorf("unknown component subtype: %q and/or model: %q", rName.Subtype, config.Model)
	}

	deps, err := draft.getDependencies(config)
	if err != nil {
		return nil, err
	}

	var newResource interface{}
	if f.Constructor != nil {
		newResource, err = f.Constructor(ctx, deps, config, draft.original.logger)
	} else {
		draft.original.logger.Warnw("using legacy constructor", "subtype", rName.Subtype, "model", config.Model)
		newResource, err = f.RobotConstructor(ctx, draft, config, draft.original.logger)
	}

	if err != nil {
		return nil, err
	}
	c := registry.ResourceSubtypeLookup(rName.Subtype)
	if c == nil || c.Reconfigurable == nil {
		return newResource, nil
	}
	return c.Reconfigurable(newResource)
}

// TransformPose will transform the pose of the requested poseInFrame to the desired frame in the robot's frame system.
func (draft *draftRobot) TransformPose(
	ctx context.Context,
	pose *referenceframe.PoseInFrame,
	dst string,
	additionalTransforms []*commonpb.Transform,
) (*referenceframe.PoseInFrame, error) {
	return draft.original.TransformPose(ctx, pose, dst, additionalTransforms)
}

// newDraftRobot returns a new draft of a robot based on the given
// original robot and the diff describing what the new robot
// should look like.
func newDraftRobot(r *localRobot, diff *config.Diff) *draftRobot {
	return &draftRobot{
		original: r,
		diff:     diff,
		manager:  r.manager.Clone(),
		removals: newResourceManager(r.manager.opts, r.logger),
	}
}

// Rollback rolls back any intermediate changes made.
func (draft *draftRobot) Rollback(ctx context.Context) error {
	return draft.manager.Close(ctx)
}

// ProcessAndCommit processes all changes in an all-or-nothing fashion
// and then finally commits them; otherwise any changes made along the
// way are rolled back.
func (draft *draftRobot) ProcessAndCommit(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			if rollbackErr := draft.Rollback(ctx); rollbackErr != nil {
				err = multierr.Combine(err, errors.Wrap(rollbackErr, "error rolling back draft changes"))
			}
		}
	}()

	if err := draft.Process(ctx); err != nil {
		return errors.Wrap(err, "error processing draft changes")
	}

	draft.original.logger.Info("committing draft changes")
	if err := draft.Commit(ctx); err != nil {
		return errors.Wrap(err, "error committing draft changes")
	}
	return nil
}

func (draft *draftRobot) clearLeftovers(ctx context.Context) error {
	var allErrs error
	for _, p := range draft.leftovers.ReplacedProcesses {
		allErrs = multierr.Combine(allErrs, p.Stop())
	}
	return allErrs
}

// Commit commits all changes and updates the original
// robot in place.
func (draft *draftRobot) Commit(ctx context.Context) error {
	draft.original.mu.Lock()
	defer draft.original.mu.Unlock()
	if err := draft.clearLeftovers(ctx); err != nil {
		return err
	}
	if err := draft.removals.Close(ctx); err != nil {
		return err
	}
	draft.original.manager = draft.manager
	draft.original.config = draft.diff.Right
	return nil
}

// Process processes all types changes into the draft robot.
func (draft *draftRobot) Process(ctx context.Context) error {
	var err error
	if err = draft.ProcessRemoveChanges(ctx); err != nil {
		return err
	}

	draft.leftovers, err = draft.manager.UpdateConfig(ctx, draft.diff.Added, draft.diff.Modified, draft.original.logger, draft)
	if err != nil {
		return err
	}
	return nil
}

// ProcessRemoveChanges processes only subtractive changes.
func (draft *draftRobot) ProcessRemoveChanges(ctx context.Context) error {
	filtered, err := draft.manager.FilterFromConfig(ctx, draft.diff.Removed, draft.original.logger)
	if err != nil {
		return err
	}
	draft.removals = filtered
	return nil
}
