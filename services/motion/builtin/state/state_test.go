package state_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
	"go.viam.com/rdk/spatialmath"
)

var replanReason = "replanning"

func TestState(t *testing.T) {
	logger := logging.NewTestLogger(t)
	myBase := base.Named("mybase")
	t.Parallel()
	ctx := context.Background()

	t.Run("creating & stopping a state with no intermediary calls", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
	})

	t.Run("starting a new execution & stopping the state", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("starting & stopping an execution & stopping the state", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()

		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("stopping an execution is idempotnet", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("stopping the state is idempotnet", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)

		s.Stop()
		s.Stop()
	})

	t.Run("stopping an execution after stopping the state", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)

		s.Stop()

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("querying for an unknown resource returns an unknown resource error", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		_, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)
		_, err = s.PlanHistory(motion.PlanHistoryReq{})
		test.That(t, err, test.ShouldBeError, state.ErrUnknownResource)
	})

	t.Run("end to end test", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()

		// no plan statuses as no executions have been created
		ps, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps, test.ShouldBeEmpty)

		preExecution := time.Now()
		// Failing to plan the first time results in an error
		errPlanningFailed := errors.New("some reason to fail planning")
		extra := map[string]interface{}{"new_plan_fail_reason": errPlanningFailed.Error()}
		req := state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase, Extra: extra})
		id, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeError, errPlanningFailed)
		test.That(t, id, test.ShouldResemble, uuid.Nil)

		// still no plan statuses as no executions have been created
		ps2, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps2, test.ShouldBeEmpty)

		req = state.NewExecutionReq(motion.MoveOnGlobeReq{ComponentName: myBase})
		executionID1, err := s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeNil)

		// we now have a single plan status as an execution has been created
		ps3, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ps3), test.ShouldEqual, 1)
		test.That(t, ps3[0].ExecutionID, test.ShouldResemble, executionID1)
		test.That(t, ps3[0].ComponentName, test.ShouldResemble, req.ComponentName)
		test.That(t, ps3[0].PlanID, test.ShouldNotEqual, uuid.Nil)
		test.That(t, ps3[0].Status.State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, ps3[0].Status.Reason, test.ShouldBeNil)
		test.That(t, ps3[0].Status.Timestamp.After(preExecution), test.ShouldBeTrue)

		id, err = s.StartExecution(req, nil)
		test.That(t, err, test.ShouldBeError, fmt.Errorf("there is already an active executionID: %s", executionID1))
		test.That(t, id, test.ShouldResemble, uuid.Nil)

		// Returns results if active plans are requested & there are active plans
		ps4, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{OnlyActivePlans: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps4, test.ShouldResemble, ps3)

		// We see that the component has an excution with a single plan & that plan
		// is in progress & has had no other statuses.
		pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws), test.ShouldEqual, 1)
		// plan id is the same as it was in the list status response
		test.That(t, pws[0].Plan.ID, test.ShouldResemble, ps3[0].PlanID)
		test.That(t, pws[0].Plan.ExecutionID, test.ShouldEqual, executionID1)
		test.That(t, pws[0].Plan.ComponentName, test.ShouldResemble, myBase)
		test.That(t, len(pws[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, pws[0].StatusHistory[0].Timestamp.After(preExecution), test.ShouldBeTrue)

		preStop := time.Now()
		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		ps5, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ps5), test.ShouldEqual, 1)
		test.That(t, ps5[0].ExecutionID, test.ShouldResemble, executionID1)
		test.That(t, ps5[0].ComponentName, test.ShouldResemble, req.ComponentName)
		test.That(t, ps5[0].PlanID, test.ShouldNotEqual, uuid.Nil)
		// status now shows that the plan is stopped
		test.That(t, ps5[0].Status.State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, ps5[0].Status.Reason, test.ShouldBeNil)
		test.That(t, ps5[0].Status.Timestamp.After(preStop), test.ShouldBeTrue)

		// Returns no results if active plans are requested & there are no active plans
		ps6, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{OnlyActivePlans: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps6, test.ShouldBeEmpty)

		// We after stoping execution of the base that the same execution has the same
		// plan, but that that plan's status is now stoped.
		// The prior status is still in the status history.
		pws2, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(pws2), test.ShouldEqual, 1)
		test.That(t, pws2[0].Plan, test.ShouldResemble, pws[0].Plan)
		test.That(t, len(pws2[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws2[0].StatusHistory[1], test.ShouldResemble, pws[0].StatusHistory[0])
		test.That(t, pws2[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, pws2[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, pws2[0].StatusHistory[0].Timestamp.After(pws2[0].StatusHistory[1].Timestamp), test.ShouldBeTrue)

		preExecution2 := time.Now()
		tc2 := &state.TestConfig{
			ReplanRequestChan:     make(chan state.ReplanRequest),
			ReplanResponseChan:    make(chan struct{}),
			ExecutionRequestChan:  make(chan state.ExecutionRequest),
			ExecutionResponseChan: make(chan struct{}),
		}
		executionID2, err := s.StartExecution(req, tc2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID2, test.ShouldNotResemble, executionID1)

		// We see after starting a new execution that the old execution is no longer returned and that a new plan has been generated
		pws4, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws4), test.ShouldEqual, 1)
		test.That(t, pws4[0].Plan.ID, test.ShouldNotResemble, pws2[0].Plan.ID)
		test.That(t, pws4[0].Plan.ExecutionID, test.ShouldNotResemble, pws2[0].Plan.ExecutionID)
		test.That(t, len(pws4[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, pws4[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws4[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, pws4[0].StatusHistory[0].Timestamp.After(preExecution2), test.ShouldBeTrue)

		// trigger replanning once
		execution2Replan1 := time.Now()
		tc2.ReplanRequestChan <- state.ReplanRequest{}
		<-tc2.ReplanResponseChan

		pws5, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws5), test.ShouldEqual, 2)
		// Previous plan is moved to higher index
		test.That(t, pws5[1].Plan, test.ShouldResemble, pws4[0].Plan)
		// Current plan is a new plan
		test.That(t, pws5[0].Plan.ID, test.ShouldNotResemble, pws4[0].Plan.ID)
		// From the same execution (definition of a replan)
		test.That(t, pws5[0].Plan.ExecutionID, test.ShouldResemble, pws4[0].Plan.ExecutionID)
		// new current plan has an in progress status & was created after triggering replanning
		test.That(t, len(pws5[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, pws5[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws5[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, pws5[0].StatusHistory[0].Timestamp.After(execution2Replan1), test.ShouldBeTrue)
		// previous plan was moved to failed state due to replanning after replanning was triggered
		test.That(t, len(pws5[1].StatusHistory), test.ShouldEqual, 2)
		// oldest satus of previous plan is unchanged, just at a higher index
		test.That(t, pws5[1].StatusHistory[1], test.ShouldResemble, pws4[0].StatusHistory[0])
		// last status of the previous plan is failed due to replanning & occurred after replanning was triggered
		test.That(t, pws5[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, pws5[1].StatusHistory[0].Reason, test.ShouldNotBeNil)
		test.That(t, *pws5[1].StatusHistory[0].Reason, test.ShouldResemble, replanReason)
		test.That(t, pws5[1].StatusHistory[0].Timestamp.After(execution2Replan1), test.ShouldBeTrue)

		// only the last plan is returned if LastPlanOnly is true
		pws6, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, LastPlanOnly: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws6), test.ShouldEqual, 1)
		test.That(t, pws6[0], test.ShouldResemble, pws5[0])

		// only the last plan is returned if LastPlanOnly is true
		// and the execution id is provided which matches the last execution for the component
		pws7, err := s.PlanHistory(motion.PlanHistoryReq{
			ComponentName: myBase,
			LastPlanOnly:  true,
			ExecutionID:   pws6[0].Plan.ExecutionID,
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pws7, test.ShouldResemble, pws6)

		// Succeeded status
		preSuccessMsg := time.Now()
		tc2.ExecutionRequestChan <- state.ExecutionRequest{}
		<-tc2.ExecutionResponseChan
		pws8, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws8), test.ShouldEqual, 2)
		// last plan is unchanged
		test.That(t, pws8[1], test.ShouldResemble, pws5[1])
		// current plan is the same as it was before
		test.That(t, pws8[0].Plan, test.ShouldResemble, pws6[0].Plan)
		// current plan now has a new status
		test.That(t, len(pws8[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws8[0].StatusHistory[1], test.ShouldResemble, pws6[0].StatusHistory[0])
		// new status is succeeded
		test.That(t, pws8[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
		test.That(t, pws8[0].StatusHistory[0].Reason, test.ShouldBeNil)
		test.That(t, pws8[0].StatusHistory[0].Timestamp.After(preSuccessMsg), test.ShouldBeTrue)

		// Failed after replanning
		tc3 := &state.TestConfig{
			ReplanRequestChan:     make(chan state.ReplanRequest),
			ReplanResponseChan:    make(chan struct{}),
			ExecutionRequestChan:  make(chan state.ExecutionRequest),
			ExecutionResponseChan: make(chan struct{}),
		}
		preExecution3 := time.Now()
		executionID3, err := s.StartExecution(req, tc3)
		test.That(t, err, test.ShouldBeNil)

		// first replan succeeds
		execution3Replan1 := time.Now()
		tc3.ReplanRequestChan <- state.ReplanRequest{}
		<-tc3.ReplanResponseChan

		// second replan fails
		execution3Replan2 := time.Now()
		replanFailReason := "replanning failed under test"
		tc3.ReplanRequestChan <- state.ReplanRequest{FailReason: &replanFailReason}
		<-tc3.ReplanResponseChan

		pws9, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(pws9), test.ShouldEqual, 2)
		test.That(t, pws9[0].Plan.ExecutionID, test.ShouldEqual, executionID3)
		test.That(t, pws9[1].Plan.ExecutionID, test.ShouldEqual, executionID3)
		test.That(t, pws9[0].Plan.ID, test.ShouldNotEqual, pws8[1].Plan.ID)
		test.That(t, len(pws9[1].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws9[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *pws9[1].StatusHistory[0].Reason, test.ShouldResemble, replanReason)
		test.That(t, pws9[1].StatusHistory[0].Timestamp.After(execution3Replan1), test.ShouldBeTrue)
		test.That(t, pws9[1].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws9[1].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, pws9[1].StatusHistory[1].Timestamp.After(preExecution3), test.ShouldBeTrue)
		test.That(t, len(pws9[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws9[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *pws9[0].StatusHistory[0].Reason, test.ShouldResemble, replanFailReason)
		test.That(t, pws9[0].StatusHistory[0].Timestamp.After(execution3Replan2), test.ShouldBeTrue)
		test.That(t, pws9[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws9[0].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, pws9[0].StatusHistory[1].Timestamp.After(execution3Replan1), test.ShouldBeTrue)

		// Failed at the end of execution
		tc4 := &state.TestConfig{
			ReplanRequestChan:     make(chan state.ReplanRequest),
			ReplanResponseChan:    make(chan struct{}),
			ExecutionRequestChan:  make(chan state.ExecutionRequest),
			ExecutionResponseChan: make(chan struct{}),
		}
		preExecution4 := time.Now()
		executionID4, err := s.StartExecution(req, tc4)
		test.That(t, err, test.ShouldBeNil)

		// first replan succeeds
		execution4Replan := time.Now()
		tc4.ReplanRequestChan <- state.ReplanRequest{}
		<-tc4.ReplanResponseChan

		// then execution fails
		execution4ExecutionFail := time.Now()
		executionFailReason := "execution failed under test"
		tc4.ExecutionRequestChan <- state.ExecutionRequest{FailReason: &executionFailReason}
		<-tc4.ExecutionResponseChan

		pws10, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)

		test.That(t, len(pws10), test.ShouldEqual, 2)
		test.That(t, pws10[0].Plan.ExecutionID, test.ShouldEqual, executionID4)
		test.That(t, pws10[1].Plan.ExecutionID, test.ShouldEqual, executionID4)
		test.That(t, pws10[0].Plan.ID, test.ShouldNotEqual, pws9[1].Plan.ID)
		test.That(t, len(pws10[1].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws10[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *pws10[1].StatusHistory[0].Reason, test.ShouldResemble, replanReason)
		test.That(t, pws10[1].StatusHistory[0].Timestamp.After(execution4Replan), test.ShouldBeTrue)
		test.That(t, pws10[1].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws10[1].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, pws10[1].StatusHistory[1].Timestamp.After(preExecution4), test.ShouldBeTrue)
		test.That(t, len(pws10[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, pws10[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *pws10[0].StatusHistory[0].Reason, test.ShouldResemble, executionFailReason)
		test.That(t, pws10[0].StatusHistory[0].Timestamp.After(execution4ExecutionFail), test.ShouldBeTrue)
		test.That(t, pws10[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws10[0].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, pws10[0].StatusHistory[1].Timestamp.After(execution4Replan), test.ShouldBeTrue)

		// providing an executionID lets you look up the plans from a prior execution
		pws12, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, ExecutionID: executionID3})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pws12, test.ShouldResemble, pws9)

		// providing an executionID with lastPlanOnly gives you the last plan of that execution
		pws13, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, ExecutionID: executionID3, LastPlanOnly: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws13), test.ShouldEqual, 1)
		test.That(t, pws13[0], test.ShouldResemble, pws9[0])

		// providing an executionID which is not known to the state returns an error
		pws14, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, ExecutionID: uuid.New()})
		test.That(t, err, test.ShouldBeError, state.ErrNotFound)
		test.That(t, len(pws14), test.ShouldEqual, 0)

		// Returns the last status of all plans that have executed
		ps7, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ps7), test.ShouldEqual, 7)
		test.That(t, ps7[0].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[0].ExecutionID, test.ShouldResemble, executionID4)
		test.That(t, ps7[0].PlanID, test.ShouldResemble, pws10[0].Plan.ID)
		test.That(t, ps7[0].Status, test.ShouldResemble, pws10[0].StatusHistory[0])

		test.That(t, ps7[1].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[1].ExecutionID, test.ShouldResemble, executionID4)
		test.That(t, ps7[1].PlanID, test.ShouldResemble, pws10[1].Plan.ID)
		test.That(t, ps7[1].Status, test.ShouldResemble, pws10[1].StatusHistory[0])

		test.That(t, ps7[2].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[2].ExecutionID, test.ShouldResemble, executionID3)
		test.That(t, ps7[2].PlanID, test.ShouldResemble, pws9[0].Plan.ID)
		test.That(t, ps7[2].Status, test.ShouldResemble, pws9[0].StatusHistory[0])

		test.That(t, ps7[3].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[3].ExecutionID, test.ShouldResemble, executionID3)
		test.That(t, ps7[3].PlanID, test.ShouldResemble, pws9[1].Plan.ID)
		test.That(t, ps7[3].Status, test.ShouldResemble, pws9[1].StatusHistory[0])

		test.That(t, ps7[4].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[4].ExecutionID, test.ShouldResemble, executionID2)
		test.That(t, ps7[4].PlanID, test.ShouldResemble, pws8[0].Plan.ID)
		test.That(t, ps7[4].Status, test.ShouldResemble, pws8[0].StatusHistory[0])

		test.That(t, ps7[5].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[5].ExecutionID, test.ShouldResemble, executionID2)
		test.That(t, ps7[5].PlanID, test.ShouldResemble, pws8[1].Plan.ID)
		test.That(t, ps7[5].Status, test.ShouldResemble, pws8[1].StatusHistory[0])

		test.That(t, ps7[6].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[6].ExecutionID, test.ShouldResemble, executionID1)
		test.That(t, ps7[6].PlanID, test.ShouldResemble, pws2[0].Plan.ID)
		test.That(t, ps7[6].Status, test.ShouldResemble, pws2[0].StatusHistory[0])

		ps8, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{OnlyActivePlans: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps8, test.ShouldBeEmpty)
	})
}

func TestNewExecutionReq(t *testing.T) {
	b := base.Named("mybase")
	ms := movementsensor.Named("mySensor")
	geometries, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 5, Y: 50, Z: 10}, "wall")
	test.That(t, err, test.ShouldBeNil)
	gpsPoint := geo.NewPoint(-70, 40)
	mogReq := motion.MoveOnGlobeReq{
		ComponentName: b,
		Heading:       1,
		Destination:   geo.NewPoint(1, 2),
		MotionCfg: &motion.MotionConfiguration{
			PositionPollingFreqHz: 4,
			ObstaclePollingFreqHz: 1,
			PlanDeviationMM:       15,
		},
		Obstacles:          []*spatialmath.GeoObstacle{spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})},
		MovementSensorName: ms,
	}
	req := state.NewExecutionReq(mogReq)
	test.That(t, mogReq.ComponentName, test.ShouldResemble, req.ComponentName)
	test.That(t, mogReq.Heading, test.ShouldAlmostEqual, req.Heading)
	test.That(t, mogReq.Destination, test.ShouldResemble, req.Destination)
	test.That(t, mogReq.MotionCfg, test.ShouldResemble, req.MotionCfg)
	test.That(t, mogReq.Obstacles, test.ShouldResemble, req.Obstacles)
	test.That(t, mogReq.MovementSensorName, test.ShouldResemble, req.MovementSensorName)
}
