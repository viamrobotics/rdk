package state_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
)

var errReplanReason = errors.New("replan triggered due to location drift")

// testPlannerExecutor is a mock PlanExecutor implementation.
type testPlannerExecutor struct {
	planFunc    func() (state.PlanResp, error)
	executeFunc func(state.Waypoints) (bool, error)
}

// by default Plan successfully returns an empty plan.
func (tpe *testPlannerExecutor) Plan() (state.PlanResp, error) {
	if tpe.planFunc != nil {
		return tpe.planFunc()
	}
	return state.PlanResp{}, nil
}

// by default Execute returns a success response.
func (tpe *testPlannerExecutor) Execute(wp state.Waypoints) (bool, error) {
	if tpe.executeFunc != nil {
		return tpe.executeFunc(wp)
	}
	return false, nil
}

func TestState(t *testing.T) {
	logger := logging.NewTestLogger(t)
	myBase := base.Named("mybase")
	t.Parallel()

	executionWaitingForCtxCancelledPlanConstructor := func(
		ctx context.Context,
		req motion.MoveOnGlobeReq,
		seedPlan motionplan.Plan,
		replanCount int,
	) (state.PlannerExecutor, error) {
		return &testPlannerExecutor{executeFunc: func(wp state.Waypoints) (bool, error) {
			<-ctx.Done()
			return false, context.Cause(ctx)
		}}, nil
	}

	successPlanConstructor := func(
		ctx context.Context,
		req motion.MoveOnGlobeReq,
		seedPlan motionplan.Plan,
		replanCount int,
	) (state.PlannerExecutor, error) {
		return &testPlannerExecutor{executeFunc: func(wp state.Waypoints) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return false, nil
		}}, nil
	}

	replanPlanConstructor := func(
		ctx context.Context,
		req motion.MoveOnGlobeReq,
		seedPlan motionplan.Plan,
		replanCount int,
	) (state.PlannerExecutor, error) {
		return &testPlannerExecutor{executeFunc: func(wp state.Waypoints) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return true, errReplanReason
		}}, nil
	}

	failedExecutionPlanConstructor := func(
		ctx context.Context,
		_ motion.MoveOnGlobeReq,
		_ motionplan.Plan,
		_ int,
	) (state.PlannerExecutor, error) {
		return &testPlannerExecutor{executeFunc: func(wp state.Waypoints) (bool, error) {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return false, errors.New("execution failed")
		}}, nil
	}

	//nolint:unparam
	failedPlanningPlanConstructor := func(
		ctx context.Context,
		_ motion.MoveOnGlobeReq,
		_ motionplan.Plan,
		_ int,
	) (state.PlannerExecutor, error) {
		return &testPlannerExecutor{
			planFunc: func() (state.PlanResp, error) {
				return state.PlanResp{}, errors.New("planning failed")
			},
			executeFunc: func(wp state.Waypoints) (bool, error) {
				t.Log("should not be called as planning failed")
				t.FailNow()

				if err := ctx.Err(); err != nil {
					return false, err
				}
				return true, errReplanReason
			},
		}, nil
	}

	failedReplanningPlanConstructor := func(
		ctx context.Context,
		_ motion.MoveOnGlobeReq,
		_ motionplan.Plan,
		replanCount int,
	) (state.PlannerExecutor, error) {
		// first replan fails during planning
		if replanCount == 1 {
			return &testPlannerExecutor{
				planFunc: func() (state.PlanResp, error) {
					return state.PlanResp{}, errors.New("planning failed")
				},
				executeFunc: func(wp state.Waypoints) (bool, error) {
					if err := ctx.Err(); err != nil {
						return false, err
					}
					return true, errReplanReason
				},
			}, nil
		}
		// first plan generates a plan but execution triggers a replan
		return &testPlannerExecutor{
			executeFunc: func(wp state.Waypoints) (bool, error) {
				if err := ctx.Err(); err != nil {
					return false, err
				}
				return true, errReplanReason
			},
		}, nil
	}

	emptyReq := motion.MoveOnGlobeReq{ComponentName: myBase}
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
		_, err := state.StartExecution(s, emptyReq.ComponentName, emptyReq, successPlanConstructor)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("starting & stopping an execution & stopping the state", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()

		_, err := state.StartExecution(s, emptyReq.ComponentName, emptyReq, executionWaitingForCtxCancelledPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.StartExecution(s, emptyReq.ComponentName, emptyReq, successPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.StartExecution(s, emptyReq.ComponentName, emptyReq, replanPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.StartExecution(s, emptyReq.ComponentName, emptyReq, failedExecutionPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.StartExecution(s, emptyReq.ComponentName, emptyReq, failedPlanningPlanConstructor)
		test.That(t, err, test.ShouldBeError, errors.New("planning failed"))

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)

		_, err = state.StartExecution(s, emptyReq.ComponentName, emptyReq, failedReplanningPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("stopping an execution is idempotnet", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := motion.MoveOnGlobeReq{ComponentName: myBase}
		_, err := state.StartExecution(s, req.ComponentName, req, executionWaitingForCtxCancelledPlanConstructor)
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
		req := motion.MoveOnGlobeReq{ComponentName: myBase}
		_, err := state.StartExecution(s, req.ComponentName, req, executionWaitingForCtxCancelledPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		s.Stop()
		s.Stop()
	})

	t.Run("stopping an execution after stopping the state", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := motion.MoveOnGlobeReq{ComponentName: myBase}
		_, err := state.StartExecution(s, req.ComponentName, req, executionWaitingForCtxCancelledPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		s.Stop()

		err = s.StopExecutionByResource(myBase)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("querying for an unknown resource returns an unknown resource error", func(t *testing.T) {
		t.Parallel()
		s := state.NewState(ctx, logger)
		defer s.Stop()
		req := motion.MoveOnGlobeReq{ComponentName: myBase}
		_, err := state.StartExecution(s, req.ComponentName, req, executionWaitingForCtxCancelledPlanConstructor)
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
		req := motion.MoveOnGlobeReq{ComponentName: myBase}
		id, err := state.StartExecution(s, req.ComponentName, req, failedPlanningPlanConstructor)
		test.That(t, err, test.ShouldBeError, errors.New("planning failed"))
		test.That(t, id, test.ShouldResemble, uuid.Nil)

		// still no plan statuses as no executions have been created
		ps2, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps2, test.ShouldBeEmpty)

		req = motion.MoveOnGlobeReq{ComponentName: myBase}
		executionID1, err := state.StartExecution(s, req.ComponentName, req, executionWaitingForCtxCancelledPlanConstructor)
		test.That(t, err, test.ShouldBeNil)

		cancelCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*50)
		defer cancelFn()
		// poll until ListPlanStatuses response has length 1
		resPS, succ := pollUntil(cancelCtx, func() (struct {
			ps  []motion.PlanStatusWithID
			err error
		}, bool,
		) {
			st := struct {
				ps  []motion.PlanStatusWithID
				err error
			}{}
			ps, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{})
			if err == nil && len(ps) == 1 {
				st.ps = ps
				st.err = err
				return st, true
			}
			return st, false
		})

		test.That(t, succ, test.ShouldBeTrue)
		test.That(t, resPS.err, test.ShouldBeNil)
		// we now have a single plan status as an execution has been created
		test.That(t, len(resPS.ps), test.ShouldEqual, 1)
		test.That(t, resPS.ps[0].ExecutionID, test.ShouldResemble, executionID1)
		test.That(t, resPS.ps[0].ComponentName, test.ShouldResemble, req.ComponentName)
		test.That(t, resPS.ps[0].PlanID, test.ShouldNotEqual, uuid.Nil)
		test.That(t, resPS.ps[0].Status.State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPS.ps[0].Status.Reason, test.ShouldBeNil)
		test.That(t, resPS.ps[0].Status.Timestamp.After(preExecution), test.ShouldBeTrue)

		id, err = state.StartExecution(s, req.ComponentName, req, replanPlanConstructor)
		test.That(t, err, test.ShouldBeError, fmt.Errorf("there is already an active executionID: %s", executionID1))
		test.That(t, id, test.ShouldResemble, uuid.Nil)

		// Returns results if active plans are requested & there are active plans
		ps4, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{OnlyActivePlans: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps4, test.ShouldResemble, resPS.ps)

		// We see that the component has an excution with a single plan & that plan
		// is in progress & has had no other statuses.
		pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws), test.ShouldEqual, 1)
		// plan id is the same as it was in the list status response
		test.That(t, pws[0].Plan.ID, test.ShouldResemble, resPS.ps[0].PlanID)
		test.That(t, pws[0].Plan.ExecutionID, test.ShouldEqual, executionID1)
		test.That(t, pws[0].Plan.ComponentName, test.ShouldResemble, myBase)
		test.That(t, len(pws[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, pws[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, pws[0].StatusHistory[0].Timestamp.After(preExecution), test.ShouldBeTrue)
		test.That(t, planStatusTimestampsInOrder(pws[0].StatusHistory), test.ShouldBeTrue)

		preStop := time.Now()
		// stop the in progress execution
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
		// previous in progres PlanStatus is now at a higher index
		test.That(t, pws2[0].StatusHistory[1], test.ShouldResemble, pws[0].StatusHistory[0])
		// most recent PlanStatus is now that it is stopped
		test.That(t, pws2[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, pws2[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, planStatusTimestampsInOrder(pws2[0].StatusHistory), test.ShouldBeTrue)

		preExecution2 := time.Now()
		ctxReplanning, triggerReplanning := context.WithCancel(context.Background())
		ctxExecutionSuccess, triggerExecutionSuccess := context.WithCancel(context.Background())
		executionID2, err := state.StartExecution(s, req.ComponentName, req, func(
			ctx context.Context,
			req motion.MoveOnGlobeReq,
			seedPlan motionplan.Plan,
			replanCount int,
		) (state.PlannerExecutor, error) {
			return &testPlannerExecutor{
				executeFunc: func(wp state.Waypoints) (bool, error) {
					if replanCount == 0 {
						// wait for replanning
						<-ctxReplanning.Done()
						return true, errReplanReason
					}
					<-ctxExecutionSuccess.Done()
					return false, nil
				},
			}, nil
		})
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
		test.That(t, planStatusTimestampsInOrder(pws4[0].StatusHistory), test.ShouldBeTrue)

		// trigger replanning once
		execution2Replan1 := time.Now()
		triggerReplanning()

		// poll until there are 2 plans in the history
		resPWS, succ := pollUntil(cancelCtx, func() (pwsRes, bool,
		) {
			st := pwsRes{}
			pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
			if err == nil && len(pws) == 2 {
				st.pws = pws
				st.err = err
				return st, true
			}
			return st, false
		})

		test.That(t, succ, test.ShouldBeTrue)
		test.That(t, resPWS.err, test.ShouldBeNil)
		test.That(t, len(resPWS.pws), test.ShouldEqual, 2)
		// Previous plan is moved to higher index
		test.That(t, resPWS.pws[1].Plan, test.ShouldResemble, pws4[0].Plan)
		// Current plan is a new plan
		test.That(t, resPWS.pws[0].Plan.ID, test.ShouldNotResemble, pws4[0].Plan.ID)
		// From the same execution (definition of a replan)
		test.That(t, resPWS.pws[0].Plan.ExecutionID, test.ShouldResemble, pws4[0].Plan.ExecutionID)
		// new current plan has an in progress status & was created after triggering replanning
		test.That(t, len(resPWS.pws[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, resPWS.pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPWS.pws[0].StatusHistory[0].Reason, test.ShouldEqual, nil)
		test.That(t, resPWS.pws[0].StatusHistory[0].Timestamp.After(execution2Replan1), test.ShouldBeTrue)
		// previous plan was moved to failed state due to replanning after replanning was triggered
		test.That(t, len(resPWS.pws[1].StatusHistory), test.ShouldEqual, 2)
		// oldest satus of previous plan is unchanged, just at a higher index
		test.That(t, resPWS.pws[1].StatusHistory[1], test.ShouldResemble, pws4[0].StatusHistory[0])
		// last status of the previous plan is failed due to replanning & occurred after replanning was triggered
		test.That(t, resPWS.pws[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, resPWS.pws[1].StatusHistory[0].Reason, test.ShouldNotBeNil)
		test.That(t, *resPWS.pws[1].StatusHistory[0].Reason, test.ShouldResemble, errReplanReason.Error())
		test.That(t, resPWS.pws[1].StatusHistory[0].Timestamp.After(execution2Replan1), test.ShouldBeTrue)
		test.That(t, planStatusTimestampsInOrder(resPWS.pws[0].StatusHistory), test.ShouldBeTrue)
		test.That(t, planStatusTimestampsInOrder(resPWS.pws[1].StatusHistory), test.ShouldBeTrue)

		// only the last plan is returned if LastPlanOnly is true
		pws6, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, LastPlanOnly: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws6), test.ShouldEqual, 1)
		test.That(t, pws6[0], test.ShouldResemble, resPWS.pws[0])

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
		triggerExecutionSuccess()

		resPWS2, succ := pollUntil(cancelCtx, func() (pwsRes, bool,
		) {
			st := pwsRes{}
			pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
			if err == nil && len(pws[0].StatusHistory) == 2 {
				st.pws = pws
				st.err = err
				return st, true
			}
			return st, false
		})
		//
		test.That(t, succ, test.ShouldBeTrue)
		test.That(t, resPWS2.err, test.ShouldBeNil)
		test.That(t, len(resPWS2.pws), test.ShouldEqual, 2)
		// last plan is unchanged
		test.That(t, resPWS2.pws[1], test.ShouldResemble, resPWS.pws[1])
		// current plan is the same as it was before
		test.That(t, resPWS2.pws[0].Plan, test.ShouldResemble, pws6[0].Plan)
		// current plan now has a new status
		test.That(t, len(resPWS2.pws[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, resPWS2.pws[0].StatusHistory[1], test.ShouldResemble, pws6[0].StatusHistory[0])
		// new status is succeeded
		test.That(t, resPWS2.pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
		test.That(t, resPWS2.pws[0].StatusHistory[0].Reason, test.ShouldBeNil)
		test.That(t, resPWS2.pws[0].StatusHistory[0].Timestamp.After(preSuccessMsg), test.ShouldBeTrue)
		test.That(t, planStatusTimestampsInOrder(resPWS2.pws[0].StatusHistory), test.ShouldBeTrue)

		// // Failed after replanning
		preExecution3 := time.Now()
		replanFailReason := errors.New("replanning failed")
		executionID3, err := state.StartExecution(s, req.ComponentName, req, func(
			ctx context.Context,
			req motion.MoveOnGlobeReq,
			seedPlan motionplan.Plan,
			replanCount int,
		) (state.PlannerExecutor, error) {
			return &testPlannerExecutor{
				planFunc: func() (state.PlanResp, error) {
					// first plan succeeds
					if replanCount == 0 {
						return state.PlanResp{}, nil
					}
					// first replan succeeds
					if replanCount == 1 {
						return state.PlanResp{}, nil
					}
					// second replan fails
					return state.PlanResp{}, replanFailReason
				},
				executeFunc: func(wp state.Waypoints) (bool, error) {
					if replanCount == 0 {
						return true, errReplanReason
					}
					if replanCount == 1 {
						return true, errReplanReason
					}
					t.Log("shouldn't execute as first replanning fails")
					t.FailNow()
					return false, nil
				},
			}, nil
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID2, test.ShouldNotResemble, executionID1)

		resPWS3, succ := pollUntil(cancelCtx, func() (pwsRes, bool,
		) {
			st := pwsRes{}
			pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
			if err == nil && len(pws) == 2 && len(pws[0].StatusHistory) == 2 {
				st.pws = pws
				st.err = err
				return st, true
			}
			return st, false
		})

		test.That(t, succ, test.ShouldBeTrue)
		test.That(t, resPWS3.err, test.ShouldBeNil)

		test.That(t, len(resPWS3.pws), test.ShouldEqual, 2)
		test.That(t, resPWS3.pws[0].Plan.ExecutionID, test.ShouldEqual, executionID3)
		test.That(t, resPWS3.pws[1].Plan.ExecutionID, test.ShouldEqual, executionID3)
		test.That(t, resPWS3.pws[0].Plan.ID, test.ShouldNotEqual, resPWS2.pws[1].Plan.ID)
		test.That(t, len(resPWS3.pws[1].StatusHistory), test.ShouldEqual, 2)
		test.That(t, resPWS3.pws[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *resPWS3.pws[1].StatusHistory[0].Reason, test.ShouldResemble, errReplanReason.Error())
		test.That(t, resPWS3.pws[1].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPWS3.pws[1].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, resPWS3.pws[1].StatusHistory[1].Timestamp.After(preExecution3), test.ShouldBeTrue)
		test.That(t, len(resPWS3.pws[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, resPWS3.pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *resPWS3.pws[0].StatusHistory[0].Reason, test.ShouldResemble, replanFailReason.Error())
		test.That(t, resPWS3.pws[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPWS3.pws[0].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, planStatusTimestampsInOrder(resPWS3.pws[0].StatusHistory), test.ShouldBeTrue)
		test.That(t, planStatusTimestampsInOrder(resPWS3.pws[1].StatusHistory), test.ShouldBeTrue)

		// Failed at the end of execution
		preExecution4 := time.Now()
		executionFailReason := errors.New("execution failed")
		executionID4, err := state.StartExecution(s, req.ComponentName, req, func(
			ctx context.Context,
			req motion.MoveOnGlobeReq,
			seedPlan motionplan.Plan,
			replanCount int,
		) (state.PlannerExecutor, error) {
			return &testPlannerExecutor{
				executeFunc: func(wp state.Waypoints) (bool, error) {
					if replanCount == 0 {
						return true, errReplanReason
					}
					return false, executionFailReason
				},
			}, nil
		})
		test.That(t, err, test.ShouldBeNil)

		resPWS4, succ := pollUntil(cancelCtx, func() (pwsRes, bool,
		) {
			st := pwsRes{}
			pws, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase})
			if err == nil && len(pws) == 2 && len(pws[0].StatusHistory) == 2 {
				st.pws = pws
				st.err = err
				return st, true
			}
			return st, false
		})

		test.That(t, succ, test.ShouldBeTrue)
		test.That(t, resPWS4.err, test.ShouldBeNil)

		test.That(t, len(resPWS4.pws), test.ShouldEqual, 2)
		test.That(t, resPWS4.pws[0].Plan.ExecutionID, test.ShouldEqual, executionID4)
		test.That(t, resPWS4.pws[1].Plan.ExecutionID, test.ShouldEqual, executionID4)
		test.That(t, resPWS4.pws[0].Plan.ID, test.ShouldNotEqual, resPWS3.pws[1].Plan.ID)
		test.That(t, len(resPWS4.pws[1].StatusHistory), test.ShouldEqual, 2)
		test.That(t, resPWS4.pws[1].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *resPWS4.pws[1].StatusHistory[0].Reason, test.ShouldResemble, errReplanReason.Error())
		test.That(t, resPWS4.pws[1].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPWS4.pws[1].StatusHistory[1].Reason, test.ShouldBeNil)
		test.That(t, resPWS4.pws[1].StatusHistory[1].Timestamp.After(preExecution4), test.ShouldBeTrue)
		test.That(t, len(resPWS4.pws[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, resPWS4.pws[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateFailed)
		test.That(t, *resPWS4.pws[0].StatusHistory[0].Reason, test.ShouldResemble, executionFailReason.Error())
		test.That(t, resPWS4.pws[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, resPWS4.pws[0].StatusHistory[1].Reason, test.ShouldBeNil)

		// providing an executionID lets you look up the plans from a prior execution
		pws12, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, ExecutionID: executionID3})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pws12, test.ShouldResemble, resPWS3.pws)

		// providing an executionID with lastPlanOnly gives you the last plan of that execution
		pws13, err := s.PlanHistory(motion.PlanHistoryReq{ComponentName: myBase, ExecutionID: executionID3, LastPlanOnly: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(pws13), test.ShouldEqual, 1)
		test.That(t, pws13[0], test.ShouldResemble, resPWS3.pws[0])

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
		test.That(t, ps7[0].PlanID, test.ShouldResemble, resPWS4.pws[0].Plan.ID)
		test.That(t, ps7[0].Status, test.ShouldResemble, resPWS4.pws[0].StatusHistory[0])

		test.That(t, ps7[1].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[1].ExecutionID, test.ShouldResemble, executionID4)
		test.That(t, ps7[1].PlanID, test.ShouldResemble, resPWS4.pws[1].Plan.ID)
		test.That(t, ps7[1].Status, test.ShouldResemble, resPWS4.pws[1].StatusHistory[0])

		test.That(t, ps7[2].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[2].ExecutionID, test.ShouldResemble, executionID3)
		test.That(t, ps7[2].PlanID, test.ShouldResemble, resPWS3.pws[0].Plan.ID)
		test.That(t, ps7[2].Status, test.ShouldResemble, resPWS3.pws[0].StatusHistory[0])

		test.That(t, ps7[3].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[3].ExecutionID, test.ShouldResemble, executionID3)
		test.That(t, ps7[3].PlanID, test.ShouldResemble, resPWS3.pws[1].Plan.ID)
		test.That(t, ps7[3].Status, test.ShouldResemble, resPWS3.pws[1].StatusHistory[0])

		test.That(t, ps7[4].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[4].ExecutionID, test.ShouldResemble, executionID2)
		test.That(t, ps7[4].PlanID, test.ShouldResemble, resPWS2.pws[0].Plan.ID)
		test.That(t, ps7[4].Status, test.ShouldResemble, resPWS2.pws[0].StatusHistory[0])

		test.That(t, ps7[5].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[5].ExecutionID, test.ShouldResemble, executionID2)
		test.That(t, ps7[5].PlanID, test.ShouldResemble, resPWS2.pws[1].Plan.ID)
		test.That(t, ps7[5].Status, test.ShouldResemble, resPWS2.pws[1].StatusHistory[0])

		test.That(t, ps7[6].ComponentName, test.ShouldResemble, myBase)
		test.That(t, ps7[6].ExecutionID, test.ShouldResemble, executionID1)
		test.That(t, ps7[6].PlanID, test.ShouldResemble, pws2[0].Plan.ID)
		test.That(t, ps7[6].Status, test.ShouldResemble, pws2[0].StatusHistory[0])

		ps8, err := s.ListPlanStatuses(motion.ListPlanStatusesReq{OnlyActivePlans: true})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps8, test.ShouldBeEmpty)
	})
}

// func planHistoryStr(pws []motion.PlanWithStatus) string {
// 	var b bytes.Buffer
// 	if len(pws) == 0 {
// 		return ""
// 	}

// 	b.WriteString(fmt.Sprintf("componentName: %s, executionID: %s", pws[0].Plan.ComponentName, pws[0].Plan.ExecutionID))

// 	for _, p := range pws {
// 		b.WriteString("\n")
// 		b.WriteString(fmt.Sprintf("planID: %s\n", p.Plan.ID))
// 		for _, s := range p.StatusHistory {
// 			if s.Reason != nil {
// 				b.WriteString(fmt.Sprintf("state: %s, reason: %s, time: %s\n", s.State, *s.Reason, s.Timestamp))
// 				continue
// 			}
// 			b.WriteString(fmt.Sprintf("state: %s, time: %s\n", s.State, s.Timestamp))
// 		}
// 	}
// 	return b.String()
// }

func planStatusTimestampsInOrder(ps []motion.PlanStatus) bool {
	if len(ps) == 0 {
		return true
	}
	last := ps[0].Timestamp
	for _, p := range ps[1:] {
		if p.Timestamp.Equal(last) || p.Timestamp.After(last) {
			return false
		}
	}
	return true
}

type pwsRes struct {
	pws []motion.PlanWithStatus
	err error
}

// pollUntil polls the funcion f returns a type T and a success boolean
// pollUntil returns when either the ctx is cancelled or f returns success = true.
// this is needed so the tests can wait until the state has been updated with the results
// of the PlanExecutor interface methods.
func pollUntil[T any](ctx context.Context, f func() (T, bool)) (T, bool) {
	t, b := f()
	for {
		if err := ctx.Err(); err != nil {
			return t, b
		}

		t, b = f()
		if b {
			return t, b
		}
	}
}
