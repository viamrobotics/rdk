// Package state provides apis for motion builtin plan executions
// and manages the state of those executions
package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
)

// Waypoints represent the waypoints of the plan.
type Waypoints [][]referenceframe.Input

// PlanResponse is the response from Plan.
type PlanResponse struct {
	Waypoints        Waypoints
	Motionplan       motionplan.Plan
	PosesByComponent []motion.PlanStep
}

// ExecuteResponse is the response from Execute.
type ExecuteResponse struct {
	// If true, the Execute function didn't reach the goal & the caller should replan
	Replan bool
	// Set if Replan is true, describes why replanning was triggered
	ReplanReason string
}

// PlannerExecutorConstructor creates a PlannerExecutor
// if ctx is cancelled then all PlannerExecutor interface
// methods must terminate & return errors
// req is the request that will be used during planning & execution
// seedPlan (nil during the first plan) is the previous plan
// if replanning has occurred
// replanCount is the number of times replanning has occurred,
// zero the first time planning occurs.
// R is a genric type which is able to be used to create a PlannerExecutor.
type PlannerExecutorConstructor[R any] func(
	ctx context.Context,
	req R,
	seedPlan motionplan.Plan,
	replanCount int,
) (PlannerExecutor, error)

// PlannerExecutor implements Plan and Execute.
type PlannerExecutor interface {
	Plan(ctx context.Context) (PlanResponse, error)
	Execute(context.Context, Waypoints) (ExecuteResponse, error)
}

type componentState struct {
	executionIDHistory []motion.ExecutionID
	executionsByID     map[motion.ExecutionID]stateExecution
}

type planMsg struct {
	plan       motion.Plan
	planStatus motion.PlanStatus
}

type stateUpdateMsg struct {
	componentName resource.Name
	executionID   motion.ExecutionID
	planID        motion.PlanID
	planStatus    motion.PlanStatus
}

// a stateExecution is the struct held in the state that
// holds the history of plans & plan status updates an
// execution has exprienced & the waitGroup & cancelFunc
// required to shut down an execution's goroutine.
type stateExecution struct {
	id            motion.ExecutionID
	componentName resource.Name
	waitGroup     *sync.WaitGroup
	cancelFunc    context.CancelFunc
	history       []motion.PlanWithStatus
}

func (e *stateExecution) stop() {
	e.cancelFunc()
	e.waitGroup.Wait()
}

func (cs componentState) lastExecution() stateExecution {
	return cs.executionsByID[cs.lastExecutionID()]
}

func (cs componentState) lastExecutionID() motion.ExecutionID {
	return cs.executionIDHistory[0]
}

// execution represents the state of a motion planning execution.
// it only ever exists in state.StartExecution function & the go routine created.
type execution[R any] struct {
	id                         motion.ExecutionID
	state                      *State
	waitGroup                  *sync.WaitGroup
	cancelCtx                  context.Context
	cancelFunc                 context.CancelFunc
	logger                     logging.Logger
	componentName              resource.Name
	req                        R
	plannerExecutorConstructor PlannerExecutorConstructor[R]
}

type planWithExecutor struct {
	plan            motion.Plan
	plannerExecutor PlannerExecutor
	waypoints       Waypoints
	motionplan      motionplan.Plan
}

// NewPlan creates a new motion.Plan from an execution & returns an error if one was not able to be created.
func (e *execution[R]) newPlanWithExecutor(ctx context.Context, seedPlan motionplan.Plan, replanCount int) (planWithExecutor, error) {
	pe, err := e.plannerExecutorConstructor(e.cancelCtx, e.req, seedPlan, replanCount)
	if err != nil {
		return planWithExecutor{}, err
	}
	resp, err := pe.Plan(ctx)
	if err != nil {
		return planWithExecutor{}, err
	}

	plan := motion.Plan{
		ID:            uuid.New(),
		ExecutionID:   e.id,
		ComponentName: e.componentName,
		Steps:         resp.PosesByComponent,
	}
	return planWithExecutor{plan: plan, plannerExecutor: pe, waypoints: resp.Waypoints, motionplan: resp.Motionplan}, nil
}

// Start starts an execution with a given plan.
func (e *execution[R]) start(ctx context.Context) error {
	var replanCount int
	originalPlanWithExecutor, err := e.newPlanWithExecutor(ctx, nil, replanCount)
	if err != nil {
		return err
	}
	e.logger.Info("no issues with e.newPlanWithExecutor")
	e.notifyStateNewExecution(e.toStateExecution(), originalPlanWithExecutor.plan, time.Now())
	// We need to add to both the state & execution waitgroups
	// B/c both the state & the stateExecution need to know if this
	// goroutine have termianted.
	// state.Stop() needs to wait for ALL execution goroutines to terminate before
	// returning in order to not leak.
	// Similarly stateExecution.stop(), which is called by state.StopExecutionByResource
	// needs to wait for its 1 execution go routine to termiante before returning.
	// As a result, both waitgroups need to be written to.
	e.state.waitGroup.Add(1)
	e.waitGroup.Add(1)
	utils.PanicCapturingGo(func() {
		defer e.state.waitGroup.Done()
		defer e.waitGroup.Done()
		defer e.cancelFunc()

		lastPWE := originalPlanWithExecutor
		// Exit conditions of this loop:
		// 1. The execution's context was cancelled, which happens if the state's Stop() was called or
		// StopExecutionByResource was called for this resource
		// 2. the execution succeeded
		// 3. the execution failed
		// 4. replanning failed
		for {
			resp, err := lastPWE.plannerExecutor.Execute(e.cancelCtx, lastPWE.waypoints)

			switch {
			// stoped
			case errors.Is(err, context.Canceled):
				e.notifyStatePlanStopped(lastPWE.plan, time.Now())
				return

			// failure
			case err != nil:
				e.notifyStatePlanFailed(lastPWE.plan, err.Error(), time.Now())
				return

			// success
			case !resp.Replan:
				e.notifyStatePlanSucceeded(lastPWE.plan, time.Now())
				return

			// replan
			default:
				replanCount++
				newPWE, err := e.newPlanWithExecutor(e.cancelCtx, lastPWE.motionplan, replanCount)
				// replan failed
				if err != nil {
					msg := "failed to replan for execution %s and component: %s, " +
						"due to replan reason: %s, tried setting previous plan %s " +
						"to failed due to error: %s\n"
					e.logger.Warnf(msg, e.id, e.componentName, resp.ReplanReason, lastPWE.plan.ID, err.Error())

					e.notifyStatePlanFailed(lastPWE.plan, err.Error(), time.Now())
					return
				}

				e.notifyStateReplan(lastPWE.plan, resp.ReplanReason, newPWE.plan, time.Now())
				lastPWE = newPWE
			}
		}
	})

	return nil
}

func (e *execution[R]) toStateExecution() stateExecution {
	return stateExecution{
		id:            e.id,
		componentName: e.componentName,
		waitGroup:     e.waitGroup,
		cancelFunc:    e.cancelFunc,
	}
}

func (e *execution[R]) notifyStateNewExecution(execution stateExecution, plan motion.Plan, time time.Time) {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	// NOTE: We hold the lock for both updateStateNewExecution & updateStateNewPlan to ensure no readers
	// are able to see a state where the execution exists but does not have a plan with a status.
	e.state.updateStateNewExecution(execution)
	e.state.updateStateNewPlan(planMsg{
		plan:       plan,
		planStatus: motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time},
	})
}

func (e *execution[R]) notifyStateReplan(lastPlan motion.Plan, reason string, newPlan motion.Plan, time time.Time) {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	// NOTE: We hold the lock for both updateStateNewExecution & updateStateNewPlan to ensure no readers
	// are able to see a state where the old plan is failed withou a new plan in progress during replanning
	e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        lastPlan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateFailed, Timestamp: time, Reason: &reason},
	})

	e.state.updateStateNewPlan(planMsg{
		plan:       newPlan,
		planStatus: motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time},
	})
}

func (e *execution[R]) notifyStatePlanFailed(plan motion.Plan, reason string, time time.Time) {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateFailed, Timestamp: time, Reason: &reason},
	})
}

func (e *execution[R]) notifyStatePlanSucceeded(plan motion.Plan, time time.Time) {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateSucceeded, Timestamp: time},
	})
}

func (e *execution[R]) notifyStatePlanStopped(plan motion.Plan, time time.Time) {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateStopped, Timestamp: time},
	})
}

// State is the state of the builtin motion service
// It keeps track of the builtin motion service's executions.
type State struct {
	waitGroup  *sync.WaitGroup
	cancelCtx  context.Context
	cancelFunc context.CancelFunc
	logger     logging.Logger
	// mu protects the componentStateByComponent
	mu                        sync.RWMutex
	componentStateByComponent map[resource.Name]componentState
}

// NewState creates a new state.
func NewState(ctx context.Context, logger logging.Logger) *State {
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	s := State{
		cancelCtx:                 cancelCtx,
		cancelFunc:                cancelFunc,
		waitGroup:                 &sync.WaitGroup{},
		componentStateByComponent: make(map[resource.Name]componentState),
		logger:                    logger,
	}
	return &s
}

// StartExecution creates a new execution from a state.
func StartExecution[R any](
	ctx context.Context,
	s *State,
	componentName resource.Name,
	req R,
	plannerExecutorConstructor PlannerExecutorConstructor[R],
) (motion.ExecutionID, error) {
	if s == nil {
		return uuid.Nil, errors.New("state is nil")
	}

	if err := s.ValidateNoActiveExecutionID(componentName); err != nil {
		return uuid.Nil, err
	}

	s.logger.Info("entered StartExecution")

	// the state being cancelled should cause all executions derived from that state to also be cancelled
	cancelCtx, cancelFunc := context.WithCancel(s.cancelCtx)
	e := execution[R]{
		id:                         uuid.New(),
		state:                      s,
		cancelCtx:                  cancelCtx,
		cancelFunc:                 cancelFunc,
		waitGroup:                  &sync.WaitGroup{},
		logger:                     s.logger,
		req:                        req,
		componentName:              componentName,
		plannerExecutorConstructor: plannerExecutorConstructor,
	}

	s.logger.Info("about to enter e.start")

	if err := e.start(ctx); err != nil {
		return uuid.Nil, err
	}

	return e.id, nil
}

// Stop stops all executions within the State.
func (s *State) Stop() {
	s.cancelFunc()
	s.waitGroup.Wait()
}

// StopExecutionByResource stops the active execution with a given resource name in the State.
func (s *State) StopExecutionByResource(componentName resource.Name) error {
	// Read lock held to get the execution
	s.mu.RLock()
	componentExectionState, exists := s.componentStateByComponent[componentName]

	// return error if component name is not in StateMap
	if !exists {
		s.mu.RUnlock()
		return resource.NewNotFoundError(componentName)
	}

	e, exists := componentExectionState.executionsByID[componentExectionState.lastExecutionID()]
	if !exists {
		s.mu.RUnlock()
		return resource.NewNotFoundError(componentName)
	}
	s.mu.RUnlock()

	// lock released while waiting for the execution to stop as the execution stopping requires writing to the state
	// which must take a lock
	e.stop()
	return nil
}

// PlanHistory returns the plans with statuses of the resource
// By default returns all plans from the most recent execution of the resoure
// If the ExecutionID is provided, returns the plans of the ExecutionID rather
// than the most recent execution
// If LastPlanOnly is provided then only the last plan is returned for the execution
// with the ExecutionID if it is provided, or the last execution
// for that component otherwise.
func (s *State) PlanHistory(req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.logger.Infof("s.componentStateByComponent: %v", s.componentStateByComponent)
	cs, exists := s.componentStateByComponent[req.ComponentName]
	if !exists {
		return nil, resource.NewNotFoundError(req.ComponentName)
	}

	executionID := req.ExecutionID

	// last plan only
	if req.LastPlanOnly {
		if ex := cs.lastExecution(); executionID == uuid.Nil || executionID == ex.id {
			history := make([]motion.PlanWithStatus, 1)
			copy(history, ex.history)
			return history, nil
		}

		// if executionID is provided & doesn't match the last execution for the component
		if ex, exists := cs.executionsByID[executionID]; exists {
			history := make([]motion.PlanWithStatus, 1)
			copy(history, ex.history)
			return history, nil
		}
		return nil, resource.NewNotFoundError(req.ComponentName)
	}

	// specific execution id when lastPlanOnly is NOT enabled
	if executionID != uuid.Nil {
		if ex, exists := cs.executionsByID[executionID]; exists {
			history := make([]motion.PlanWithStatus, len(ex.history))
			copy(history, ex.history)
			return history, nil
		}
		return nil, resource.NewNotFoundError(req.ComponentName)
	}

	ex := cs.lastExecution()
	history := make([]motion.PlanWithStatus, len(cs.lastExecution().history))
	copy(history, ex.history)
	return history, nil
}

// ListPlanStatuses returns the status of plans created by MoveOnGlobe requests
// that are executing OR are part of an execution which changed it state
// within the a 24HR TTL OR until the robot reinitializes.
// If OnlyActivePlans is provided, only returns plans which are in non terminal states.
func (s *State) ListPlanStatuses(req motion.ListPlanStatusesReq) ([]motion.PlanStatusWithID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	statuses := []motion.PlanStatusWithID{}
	if req.OnlyActivePlans {
		for name := range s.componentStateByComponent {
			if e, err := s.activeExecution(name); err == nil {
				statuses = append(statuses, motion.PlanStatusWithID{
					ExecutionID:   e.id,
					ComponentName: e.componentName,
					PlanID:        e.history[0].Plan.ID,
					Status:        e.history[0].StatusHistory[0],
				})
			}
		}
		return statuses, nil
	}

	for _, cs := range s.componentStateByComponent {
		for _, executionID := range cs.executionIDHistory {
			e, exists := cs.executionsByID[executionID]
			if !exists {
				return nil, errors.New("state is corrupted")
			}
			for _, pws := range e.history {
				statuses = append(statuses, motion.PlanStatusWithID{
					ExecutionID:   e.id,
					ComponentName: e.componentName,
					PlanID:        pws.Plan.ID,
					Status:        pws.StatusHistory[0],
				})
			}
		}
	}

	return statuses, nil
}

// ValidateNoActiveExecutionID returns an error if there is already an active
// Execution for the resource name within the State.
func (s *State) ValidateNoActiveExecutionID(name resource.Name) error {
	if es, err := s.activeExecution(name); err == nil {
		return fmt.Errorf("there is already an active executionID: %s", es.id)
	}
	return nil
}

func (s *State) updateStateNewExecution(newE stateExecution) {
	cs, exists := s.componentStateByComponent[newE.componentName]

	if exists {
		_, exists = cs.executionsByID[newE.id]
		if exists {
			err := fmt.Errorf("unexpected ExecutionID already exists %s", newE.id)
			s.logger.Error(err.Error())
			return
		}
		cs.executionsByID[newE.id] = newE
		cs.executionIDHistory = append([]motion.ExecutionID{newE.id}, cs.executionIDHistory...)
		s.componentStateByComponent[newE.componentName] = cs
	} else {
		s.componentStateByComponent[newE.componentName] = componentState{
			executionIDHistory: []motion.ExecutionID{newE.id},
			executionsByID:     map[motion.ExecutionID]stateExecution{newE.id: newE},
		}
	}
}

func (s *State) updateStateNewPlan(newPlan planMsg) {
	if newPlan.planStatus.State != motion.PlanStateInProgress {
		err := errors.New("handleNewPlan received a plan status other than in progress")
		s.logger.Error(err.Error())
		return
	}

	activeExecutionID := s.componentStateByComponent[newPlan.plan.ComponentName].lastExecutionID()
	if newPlan.plan.ExecutionID != activeExecutionID {
		e := "got new plan for inactive execution: active executionID %s, planID: %s, component: %s, plan executionID: %s"
		err := fmt.Errorf(e, activeExecutionID, newPlan.plan.ID, newPlan.plan.ComponentName, newPlan.plan.ExecutionID)
		s.logger.Error(err.Error())
		return
	}
	execution := s.componentStateByComponent[newPlan.plan.ComponentName].executionsByID[newPlan.plan.ExecutionID]
	pws := []motion.PlanWithStatus{{Plan: newPlan.plan, StatusHistory: []motion.PlanStatus{newPlan.planStatus}}}
	// prepend  to executions.history so that lower indices are newer
	execution.history = append(pws, execution.history...)

	s.componentStateByComponent[newPlan.plan.ComponentName].executionsByID[newPlan.plan.ExecutionID] = execution
}

func (s *State) updateStateStatusUpdate(update stateUpdateMsg) {
	switch update.planStatus.State {
	// terminal states
	case motion.PlanStateSucceeded, motion.PlanStateFailed, motion.PlanStateStopped:
	default:
		err := fmt.Errorf("unexpected PlanState %v in update %#v", update.planStatus.State, update)
		s.logger.Error(err.Error())
		return
	}
	componentExecutions, exists := s.componentStateByComponent[update.componentName]
	if !exists {
		err := errors.New("updated component doesn't exist")
		s.logger.Error(err.Error())
		return
	}
	// copy the execution
	execution := componentExecutions.executionsByID[update.executionID]
	lastPlanWithStatus := execution.history[0]
	if lastPlanWithStatus.Plan.ID != update.planID {
		err := fmt.Errorf("status update for plan %s is not for last plan: %s", update.planID, lastPlanWithStatus.Plan.ID)
		s.logger.Error(err.Error())
		return
	}
	lastPlanWithStatus.StatusHistory = append([]motion.PlanStatus{update.planStatus}, lastPlanWithStatus.StatusHistory...)
	// write updated last plan back to history
	execution.history[0] = lastPlanWithStatus
	// write the execution with the new history to the component execution state copy
	componentExecutions.executionsByID[update.executionID] = execution
	// write the component execution state copy back to the state
	s.componentStateByComponent[update.componentName] = componentExecutions
}

func (s *State) activeExecution(name resource.Name) (stateExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cs, exists := s.componentStateByComponent[name]; exists {
		es := cs.lastExecution()

		if _, exists := motion.TerminalStateSet[es.history[0].StatusHistory[0].State]; exists {
			return stateExecution{}, resource.NewNotFoundError(name)
		}
		return es, nil
	}
	return stateExecution{}, resource.NewNotFoundError(name)
}
