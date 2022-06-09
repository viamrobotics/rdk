package robotimpl

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
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
	if err := r.updateDefaultServices(ctx); err != nil {
		return err
	}
	r.manager.updateResourceRemoteNames()
	return nil
}

// A draftRobot is responsible for the workflow of turning in
// a newly proposed robot into a robot ready to be swapped in
// for an existing one. It understands how to rollback and commit
// changes as safe as it possibly can.
type draftRobot struct {
	original *localRobot
	diff     *config.Diff
	manager  *resourceManager

	// additions and removals consist of modifications as well since we treat
	// any modification as a removal to commit and an addition to rollback.
	additions     *resourceManager
	modifications *resourceManager
	removals      *resourceManager
}

// newDraftRobot returns a new draft of a robot based on the given
// original robot and the diff describing what the new robot
// should look like.
func newDraftRobot(r *localRobot, diff *config.Diff) *draftRobot {
	return &draftRobot{
		original:      r,
		diff:          diff,
		manager:       r.manager.Clone(),
		additions:     newResourceManager(r.manager.opts, r.logger),
		modifications: newResourceManager(r.manager.opts, r.logger),
		removals:      newResourceManager(r.manager.opts, r.logger),
	}
}

// Rollback rolls back any intermediate changes made.
func (draft *draftRobot) Rollback(ctx context.Context) error {
	order := draft.additions.resources.TopologicalSort()
	for _, k := range order {
		if _, ok := draft.manager.resources.Nodes[k]; !ok {
			if err := utils.TryClose(ctx, draft.additions.resources.Nodes[k]); err != nil {
				return err
			}
		}
		draft.additions.resources.Remove(k)
	}
	return draft.additions.Close(ctx)
}

// ProcessAndCommit processes all changes in an all-or-nothing fashion
// and then finally commits them; otherwise any changes made along the
// way are rolled back.
func (draft *draftRobot) ProcessAndCommit(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			draft.original.logger.Infow("rolling back draft changes due to error", "error", err)
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

// Commit commits all changes and updates the original
// robot in place.
func (draft *draftRobot) Commit(ctx context.Context) error {
	draft.original.mu.Lock()
	defer draft.original.mu.Unlock()

	addResult, err := draft.manager.MergeAdd(draft.additions)
	if err != nil {
		return err
	}
	modifyResult, err := draft.manager.MergeModify(ctx, draft.modifications, draft.diff)
	if err != nil {
		return err
	}
	draft.manager.MergeRemove(draft.removals)
	draft.original.manager = draft.manager
	draft.original.config = draft.diff.Right

	if err := addResult.Process(ctx, draft.removals); err != nil {
		draft.original.logger.Errorw("error processing add result but still committing changes", "error", err)
	}
	if err := modifyResult.Process(ctx, draft.removals); err != nil {
		draft.original.logger.Errorw("error processing modify result but still committing changes", "error", err)
	}
	if err := draft.removals.Close(ctx); err != nil {
		draft.original.logger.Errorw("error closing parts removed but still committing changes", "error", err)
	}
	return nil
}

// Process processes all types changes into the draft robot.
func (draft *draftRobot) Process(ctx context.Context) error {
	// We specifically add, modify, and remove parts of the robot
	// in order to provide the best chance of reconfiguration/compatibility.
	// This assumes the addition/modification of parts does not cause
	// any adverse effects before any removals.
	draft.additions.resources = draft.manager.resources.Clone()
	if err := draft.ProcessAddChanges(ctx); err != nil {
		return err
	}
	draft.modifications.resources = draft.additions.resources.Clone()
	if err := draft.ProcessModifyChanges(ctx); err != nil {
		return err
	}
	if err := draft.ProcessRemoveChanges(ctx); err != nil {
		return err
	}
	return nil
}

// ProcessAddChanges processes only additive changes.
func (draft *draftRobot) ProcessAddChanges(ctx context.Context) error {
	return draft.additions.processConfig(ctx, draft.diff.Added, draft.original, draft.original.logger)
}

// ProcessModifyChanges processes only modificative changes.
func (draft *draftRobot) ProcessModifyChanges(ctx context.Context) error {
	return draft.modifications.processModifiedConfig(ctx, draft.diff.Modified, draft.original, draft.original.logger)
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
