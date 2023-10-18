// Package state provides apis for motion builtin plan executions
// and manages the state of those executions
package state

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

var (
	// ErrUnknownResource indicates that the resource is not known.
	ErrUnknownResource = errors.New("unknown resource")
	// ErrNotFound indicates the entity was not found.
	ErrNotFound  = errors.New("not found")
	replanReason = "replanning"
)

type componentState struct {
	executionIDHistory []motion.ExecutionID
	executionsByID     map[motion.ExecutionID]stateExecution
}

type newPlanMsg struct {
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

// ExecutionReq is the request type for StartExecution.
type ExecutionReq struct {
	ComponentName      resource.Name
	Destination        *geo.Point
	Heading            float64
	MovementSensorName resource.Name
	Obstacles          []*spatialmath.GeoObstacle
	MotionCfg          *motion.MotionConfiguration
	Extra              map[string]interface{}
}

// NewExecutionReq creates an ExecutionReq from a motion.MoveOnGlobeReq.
func NewExecutionReq(req motion.MoveOnGlobeReq) ExecutionReq {
	return ExecutionReq{
		ComponentName:      req.ComponentName,
		Destination:        req.Destination,
		Heading:            req.Heading,
		MovementSensorName: req.MovementSensorName,
		Obstacles:          req.Obstacles,
		MotionCfg:          req.MotionCfg,
		Extra:              req.Extra,
	}
}

// execution represents the state of a motion planning execution.
// it only ever exists in state.StartExecution function & the go routine created
// its `start()` method.
type execution struct {
	id            motion.ExecutionID
	state         *State
	componentName resource.Name
	waitGroup     *sync.WaitGroup
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	testConfig    *TestConfig
	logger        logging.Logger
}

// NewPlan creates a new motion.Plan from an execution & returns an error if one was not able to be created.
func (e *execution) newPlan() (motion.Plan, error) {
	// TODO: Generate steps
	if e.testConfig.newPlanFailReason != nil {
		return motion.Plan{}, errors.New(*e.testConfig.newPlanFailReason)
	}
	return motion.Plan{ID: uuid.New(), ExecutionID: e.id, ComponentName: e.componentName}, nil
}

// Start starts an execution with a given plan.
func (e *execution) start(originalPlan motion.Plan) error {
	if err := e.notifyStateNewExecution(e.toStateExecution(), originalPlan, time.Now()); err != nil {
		return err
	}

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

		lastPlan := originalPlan

		// The exit conditions include:
		// 1. The execution's context was cancelled, which happens if the state's Stop() was called or
		// StopExecutionByResource was called for this resource
		// 2. replanning failed
		// 3. the execution failed
		// 4. the execution succeeded
		// 5. failing to notify the state struct for any reason (should never happen)
		for {
			if err := e.cancelCtx.Err(); err != nil {
				e.logger.Debugf("context done due to %s\n", err)
				return
			}

			select {
			case <-e.cancelCtx.Done():
				e.logger.Debugf("context done due to %s\n", e.cancelCtx.Err())
				return
				// simulates replanning
			case rr := <-e.testConfig.ReplanRequestChan:
				e.logger.Debug("replanning")
				if rr.FailReason != nil {
					if err := e.notifyStatePlanFailed(lastPlan, *rr.FailReason, time.Now()); err != nil {
						e.logger.Error(*rr.FailReason)
					}
					e.testConfig.ReplanResponseChan <- struct{}{}
					return
				}
				newPlan, err := e.newPlan()
				if err != nil {
					msg := "failed to replan for execution %s and component: %s, setting previous plan %s to failed due to error: %s\n"
					e.logger.Warnf(msg, e.id, e.componentName, lastPlan.ID, err.Error())

					if err := e.notifyStatePlanFailed(lastPlan, err.Error(), time.Now()); err != nil {
						e.logger.Error(err.Error())
					}
					e.testConfig.ReplanResponseChan <- struct{}{}
					return
				}

				e.logger.Debugf("updating last plan %s\n", lastPlan.ID)
				if err := e.notifyStatePlanFailed(lastPlan, replanReason, time.Now()); err != nil {
					e.logger.Error(err.Error())
					e.testConfig.ReplanResponseChan <- struct{}{}
					return
				}
				e.logger.Debugf("updating new plan %s\n", newPlan.ID.String())
				if err = e.notifyStateNewPlan(newPlan, time.Now()); err != nil {
					e.logger.Error(err.Error())
					e.testConfig.ReplanResponseChan <- struct{}{}
					return
				}
				lastPlan = newPlan
				e.testConfig.ReplanResponseChan <- struct{}{}

				// simulates plan reaching terminal state
			case er := <-e.testConfig.ExecutionRequestChan:
				if er.FailReason != nil {
					e.logger.Debugf("plan failed %#v\n", lastPlan)
					if err := e.notifyStatePlanFailed(lastPlan, *er.FailReason, time.Now()); err != nil {
						e.logger.Error(err.Error())
						e.testConfig.ExecutionResponseChan <- struct{}{}
						return
					}
					e.testConfig.ExecutionResponseChan <- struct{}{}
					return
				}

				e.logger.Debugf("plan succeeded %#v\n", lastPlan)
				if err := e.notifyStatePlanSucceeded(lastPlan, time.Now()); err != nil {
					e.logger.Error(err.Error())
					e.testConfig.ExecutionResponseChan <- struct{}{}
					return
				}
				e.testConfig.ExecutionResponseChan <- struct{}{}
				return
			}
		}
	})

	return nil
}

func (e *execution) toStateExecution() stateExecution {
	return stateExecution{
		id:            e.id,
		componentName: e.componentName,
		waitGroup:     e.waitGroup,
		cancelFunc:    e.cancelFunc,
	}
}

// NOTE: We hold the lock for both updateStateNewExecution & updateStateNewPlan to ensure no readers
// are able to see a state where the execution exists but does not have a plan with a status.
func (e *execution) notifyStateNewExecution(execution stateExecution, plan motion.Plan, time time.Time) error {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()

	if err := e.state.updateStateNewExecution(execution); err != nil {
		return err
	}

	msg := newPlanMsg{
		plan:       plan,
		planStatus: motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time},
	}
	if err := e.state.updateStateNewPlan(msg); err != nil {
		return err
	}

	return nil
}

func (e *execution) notifyStateNewPlan(plan motion.Plan, time time.Time) error {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	return e.state.updateStateNewPlan(newPlanMsg{
		plan:       plan,
		planStatus: motion.PlanStatus{State: motion.PlanStateInProgress, Timestamp: time},
	})
}

func (e *execution) notifyStatePlanFailed(plan motion.Plan, reason string, time time.Time) error {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	return e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateFailed, Timestamp: time, Reason: &reason},
	})
}

func (e *execution) notifyStatePlanSucceeded(plan motion.Plan, time time.Time) error {
	e.state.mu.Lock()
	defer e.state.mu.Unlock()
	return e.state.updateStateStatusUpdate(stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateSucceeded, Timestamp: time},
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
func (s *State) StartExecution(req ExecutionReq, tc *TestConfig) (motion.ExecutionID, error) {
	if err := s.validateNoActiveExecutionID(req.ComponentName); err != nil {
		return uuid.Nil, err
	}

	// the state being cancelled should cause all executions derived from that state to also be cancelled
	cancelCtx, cancelFunc := context.WithCancel(s.cancelCtx)
	e := execution{
		id:            uuid.New(),
		state:         s,
		componentName: req.ComponentName,
		cancelCtx:     cancelCtx,
		cancelFunc:    cancelFunc,
		waitGroup:     &sync.WaitGroup{},
		logger:        s.logger,
		testConfig:    addExtra(tc, req.Extra),
	}

	p, err := e.newPlan()
	if err != nil {
		return uuid.Nil, err
	}

	if err := e.start(p); err != nil {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	componentExectionState, exists := s.componentStateByComponent[componentName]

	// return error if component name is not in StateMap
	if !exists {
		return ErrUnknownResource
	}

	e, exists := componentExectionState.executionsByID[componentExectionState.lastExecutionID()]
	if !exists {
		return ErrNotFound
	}

	// NOTE: This is going to hold the write lock on the state until the execution terminates.
	// If the execution go routine is not checking the cancelCtx regularly, this could result in the
	// state lock being held for a long time.
	e.stop()

	msg := stateUpdateMsg{
		componentName: e.componentName,
		executionID:   e.id,
		planID:        e.history[0].Plan.ID,
		planStatus:    motion.PlanStatus{State: motion.PlanStateStopped, Timestamp: time.Now()},
	}

	if err := s.updateStateStatusStopped(msg); err != nil {
		return err
	}

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
	cs, exists := s.componentStateByComponent[req.ComponentName]
	if !exists {
		return nil, ErrUnknownResource
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
		return nil, ErrNotFound
	}

	// specific execution id when lastPlanOnly is NOT enabled
	if executionID != uuid.Nil {
		if ex, exists := cs.executionsByID[executionID]; exists {
			history := make([]motion.PlanWithStatus, len(ex.history))
			copy(history, ex.history)
			return history, nil
		}
		return nil, ErrNotFound
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

// validateNoActiveExecutionID returns an error if there is already an active
// Execution for the resource name within the State.
func (s *State) validateNoActiveExecutionID(name resource.Name) error {
	if es, err := s.activeExecution(name); err == nil {
		return fmt.Errorf("there is already an active executionID: %s", es.id)
	}
	return nil
}

func (s *State) updateStateNewExecution(newE stateExecution) error {
	cs, exists := s.componentStateByComponent[newE.componentName]

	if exists {
		_, exists = cs.executionsByID[newE.id]
		if exists {
			return fmt.Errorf("unexpected ExecutionID already exists %s", newE.id)
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
	return nil
}

func (s *State) updateStateNewPlan(newPlan newPlanMsg) error {
	if newPlan.planStatus.State != motion.PlanStateInProgress {
		return errors.New("handleNewPlan received a plan status other than in progress")
	}

	activeExecutionID := s.componentStateByComponent[newPlan.plan.ComponentName].lastExecutionID()
	if newPlan.plan.ExecutionID != activeExecutionID {
		e := "got new plan for inactive execution: active executionID %s, planID: %s, component: %s, plan executionID: %s"
		return fmt.Errorf(e, activeExecutionID, newPlan.plan.ID, newPlan.plan.ComponentName, newPlan.plan.ExecutionID)
	}
	execution := s.componentStateByComponent[newPlan.plan.ComponentName].executionsByID[newPlan.plan.ExecutionID]
	pws := []motion.PlanWithStatus{{Plan: newPlan.plan, StatusHistory: []motion.PlanStatus{newPlan.planStatus}}}
	// prepend  to executions.history so that lower indices are newer
	execution.history = append(pws, execution.history...)

	s.componentStateByComponent[newPlan.plan.ComponentName].executionsByID[newPlan.plan.ExecutionID] = execution
	return nil
}

func (s *State) updateStateStatusUpdate(update stateUpdateMsg) error {
	switch update.planStatus.State {
	// terminal states
	case motion.PlanStateSucceeded, motion.PlanStateFailed:
	default:
		return fmt.Errorf("unexpected PlanState %v in update %#v", update.planStatus.State, update)
	}
	componentExecutions, exists := s.componentStateByComponent[update.componentName]
	if !exists {
		return errors.New("updated component doesn't exist")
	}
	// copy the execution
	execution := componentExecutions.executionsByID[update.executionID]
	lastPlanWithStatus := execution.history[0]
	if lastPlanWithStatus.Plan.ID != update.planID {
		return fmt.Errorf("status update for plan %s is not for last plan: %s", update.planID, lastPlanWithStatus.Plan.ID)
	}
	lastPlanWithStatus.StatusHistory = append([]motion.PlanStatus{update.planStatus}, lastPlanWithStatus.StatusHistory...)
	// write updated last plan back to history
	execution.history[0] = lastPlanWithStatus
	// write the execution with the new history to the component execution state copy
	componentExecutions.executionsByID[update.executionID] = execution
	// write the component execution state copy back to the state
	s.componentStateByComponent[update.componentName] = componentExecutions

	return nil
}

// NOTE: This doesn't take locks as the caller already has the lock.
func (s *State) updateStateStatusStopped(update stateUpdateMsg) error {
	if update.planStatus.State != motion.PlanStateStopped {
		return fmt.Errorf("unexpected PlanState %v in update %#v", update.planStatus.State, update)
	}
	// copy the execution state of the component
	componentExecutions, exists := s.componentStateByComponent[update.componentName]
	if !exists {
		return errors.New("updated component doesn't exist")
	}
	// // set the copy's activeExecutionID to nil
	// componentExecutions.executionIDHistory = uuid.Nil
	// copy the execution
	execution := componentExecutions.executionsByID[update.executionID]
	lastPlanWithStatus := execution.history[0]
	if lastPlanWithStatus.Plan.ID != update.planID {
		return fmt.Errorf("status update for plan %s is not for last plan: %s", update.planID, lastPlanWithStatus.Plan.ID)
	}
	lastPlanWithStatus.StatusHistory = append([]motion.PlanStatus{update.planStatus}, lastPlanWithStatus.StatusHistory...)
	// write updated last plan back to history
	execution.history[0] = lastPlanWithStatus
	// write the execution with the new history to the component execution state copy
	componentExecutions.executionsByID[update.executionID] = execution
	// write the component execution state copy back to the state
	s.componentStateByComponent[update.componentName] = componentExecutions

	return nil
}

func (s *State) activeExecution(name resource.Name) (stateExecution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cs, exists := s.componentStateByComponent[name]; exists {
		es := cs.lastExecution()

		if _, exists := motion.TerminalStateSet[es.history[0].StatusHistory[0].State]; exists {
			return stateExecution{}, ErrNotFound
		}
		return es, nil
	}
	return stateExecution{}, ErrUnknownResource
}

// ReplanRequest is a temporary struct that triggers the execution to replan
// if FailReason is non nil then the replanning will fail.
type ReplanRequest struct {
	FailReason *string
}

// ExecutionRequest is a temporary struct that triggers the end of an execution
// if FailReason is non nil then the execution will fail.
type ExecutionRequest struct {
	// if nil then the execution succeeded
	FailReason *string
}

// TestConfig is a temporary struct that holds channels for testing execution replanning, success & failure.
type TestConfig struct {
	// ReplanRequestChan is the channel written to trigger replanning
	ReplanRequestChan chan ReplanRequest
	// ReplanResponseChan is written to by the execution after replanning has completed
	ReplanResponseChan chan struct{}
	// ExecutionRequestChan is the channel written to trigger execution termination
	ExecutionRequestChan chan ExecutionRequest
	// ExecutionResponseChan is written to by the execution after the execution has terminated
	ExecutionResponseChan chan struct{}
	newPlanFailReason     *string
}

func addExtra(tc *TestConfig, extra map[string]interface{}) *TestConfig {
	if tc == nil {
		tc = &TestConfig{}
	}
	for k, v := range extra {
		if k == "new_plan_fail_reason" {
			reason, ok := v.(string)
			if ok {
				tc.newPlanFailReason = &reason
			}
		}
	}

	return tc
}
