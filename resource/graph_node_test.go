package resource_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

func withTestLogger(t *testing.T, node *resource.GraphNode) *resource.GraphNode {
	logger := logging.NewTestLogger(t)
	node.InitializeLogger(logger, "testnode")
	return node
}

func TestUninitializedLifecycle(t *testing.T) {
	// empty
	node := withTestLogger(t, resource.NewUninitializedNode())
	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	test.That(t, node.UpdatedAt(), test.ShouldEqual, 0)
	_, err := node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	_, err = node.UnsafeResource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.Model{})
	test.That(t, node.HasResource(), test.ShouldBeFalse)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{})
	test.That(t, node.NeedsReconfigure(), test.ShouldBeFalse)

	expectedState := resource.NodeStateUnconfigured
	test.That(t, node.State(), test.ShouldEqual, expectedState)

	status := node.Status()
	test.That(t, status.State, test.ShouldResemble, expectedState)

	lifecycleTest(t, node, []string(nil))
}

func TestUnconfiguredLifecycle(t *testing.T) {
	someConf := resource.Config{
		Name:       "foo",
		Attributes: utils.AttributeMap{"3": 4},
	}
	initialDeps := []string{"dep1", "dep2"}
	node := withTestLogger(t, resource.NewUnconfiguredGraphNode(someConf, initialDeps))

	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	test.That(t, node.UpdatedAt(), test.ShouldEqual, 0)
	_, err := node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	_, err = node.UnsafeResource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.Model{})
	test.That(t, node.HasResource(), test.ShouldBeFalse)
	test.That(t, node.Config(), test.ShouldResemble, someConf)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, initialDeps)

	expectedState := resource.NodeStateConfiguring
	test.That(t, node.State(), test.ShouldEqual, expectedState)

	status := node.Status()
	test.That(t, status.Name.Name, test.ShouldEqual, "")
	test.That(t, status.State, test.ShouldResemble, expectedState)

	lifecycleTest(t, node, initialDeps)
}

func TestConfiguredLifecycle(t *testing.T) {
	someConf := resource.Config{Attributes: utils.AttributeMap{"3": 4}}

	resName := generic.Named("some")
	ourRes := &someResource{Resource: testutils.NewUnimplementedResource(resName)}
	node := withTestLogger(t, resource.NewConfiguredGraphNode(someConf, ourRes, resource.DefaultModelFamily.WithModel("bar")))

	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	test.That(t, node.UpdatedAt(), test.ShouldEqual, 0)
	res, err := node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, res)
	res, err = node.UnsafeResource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, res)
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bar"))
	test.That(t, node.HasResource(), test.ShouldBeTrue)
	test.That(t, node.Config(), test.ShouldResemble, someConf)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeFalse)
	test.That(t, node.UnresolvedDependencies(), test.ShouldBeEmpty)

	expectedState := resource.NodeStateReady
	test.That(t, node.State(), test.ShouldEqual, expectedState)

	status := node.Status()
	test.That(t, status.Name, test.ShouldResemble, resource.Name{})
	test.That(t, status.State, test.ShouldResemble, resource.NodeStateReady)

	lifecycleTest(t, node, []string(nil))
}

func lifecycleTest(t *testing.T, node *resource.GraphNode, initialDeps []string) {
	// get initial state
	state, transitionedAt := node.State(), node.TransitionedAt()

	verifyStateTransition := func(t *testing.T, node *resource.GraphNode, expectedState resource.NodeState) {
		t.Helper()

		toState, toTransitionedAt := node.State(), node.TransitionedAt()
		test.That(t, toState, test.ShouldEqual, expectedState)
		test.That(t, toTransitionedAt.UnixNano(), test.ShouldBeGreaterThan, transitionedAt.UnixNano())

		state, transitionedAt = toState, toTransitionedAt
	}

	verifySameState := func(t *testing.T, node *resource.GraphNode) {
		t.Helper()

		toState, toTransitionedAt := node.State(), node.TransitionedAt()
		test.That(t, toState, test.ShouldEqual, state)
		test.That(t, toTransitionedAt.UnixNano(), test.ShouldEqual, transitionedAt.UnixNano())
		state, transitionedAt = toState, toTransitionedAt
	}

	// mark it as [NodeStateUnhealthy]
	ourErr := errors.New("whoops")
	node.LogAndSetLastError(ourErr)
	_, err := node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	verifyStateTransition(t, node, resource.NodeStateUnhealthy)

	// mark it for removal
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	node.MarkForRemoval()
	test.That(t, node.MarkedForRemoval(), test.ShouldBeTrue)

	// Attempt to change status to [NodeStateUnhealthy]
	ourErr = errors.New("whoops")
	node.LogAndSetLastError(ourErr)
	status := node.Status()
	// Ensure that error is set and node stays in [NodeStateUnhealthy]
	// since state transition [NodeStateUnhealthy] -> [NodeStateRemoving] is blocked
	test.That(t, status.Error.Error(), test.ShouldContainSubstring, "whoops")
	test.That(t, node.MarkedForRemoval(), test.ShouldBeTrue)

	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "pending removal")

	verifyStateTransition(t, node, resource.NodeStateRemoving)

	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, initialDeps)

	// but we end up configuring it
	ourRes := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foo"))}
	node.SwapResource(ourRes, resource.DefaultModelFamily.WithModel("bar"), nil)
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bar"))
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)

	res, err := node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)

	verifyStateTransition(t, node, resource.NodeStateReady)

	// now it needs update
	node.SetNeedsUpdate()
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)

	verifyStateTransition(t, node, resource.NodeStateConfiguring)

	// but an error happened
	node.LogAndSetLastError(ourErr)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, ourErr.Error())
	res, err = node.UnsafeResource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)

	verifyStateTransition(t, node, resource.NodeStateUnhealthy)

	// it reconfigured
	ourRes2 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foo"))}
	node.SwapResource(ourRes2, resource.DefaultModelFamily.WithModel("baz"), nil)
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("baz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)

	verifyStateTransition(t, node, resource.NodeStateReady)

	// it needs a new config
	ourConf := resource.Config{Attributes: utils.AttributeMap{"1": 2}}
	node.SetNewConfig(ourConf, []string{"3", "4", "5"})
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, []string{"3", "4", "5"})
	verifyStateTransition(t, node, resource.NodeStateConfiguring)
	node.SetNeedsUpdate() // noop
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, []string{"3", "4", "5"})
	verifySameState(t, node)

	// but an error happened
	node.LogAndSetLastError(ourErr)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, ourErr.Error())
	res, err = node.UnsafeResource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes2)
	verifyStateTransition(t, node, resource.NodeStateUnhealthy)

	// it reconfigured
	ourRes3 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("fooa"))}
	node.SwapResource(ourRes3, resource.DefaultModelFamily.WithModel("bazz"), nil)
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bazz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes2)
	test.That(t, res, test.ShouldEqual, ourRes3)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldBeEmpty)
	verifyStateTransition(t, node, resource.NodeStateReady)

	//nolint
	test.That(t, node.Close(context.WithValue(context.Background(), "foo", "hi")), test.ShouldBeNil)
	test.That(t, ourRes.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes2.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes3.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes3.closeCap, test.ShouldResemble, []interface{}{"hi"})

	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	// TODO(RSDK-7928): we might want to make this transition a node to an "unconfigured"
	// or "removing" state.
	verifySameState(t, node)

	ourRes4 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foob")), shouldErr: true}
	node.SwapResource(ourRes4, resource.DefaultModelFamily.WithModel("bazzz"), nil)
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bazzz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes3)
	test.That(t, res, test.ShouldEqual, ourRes4)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	verifySameState(t, node)

	//nolint
	err = node.Close(context.WithValue(context.Background(), "foo", "bye"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bad close")
	test.That(t, ourRes.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes2.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes3.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes4.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes4.closeCap, test.ShouldResemble, []interface{}{"bye"})

	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
	// TODO(RSDK-7928): we might want to make this transition a node to an "unconfigured"
	// or "removing" state.
	verifySameState(t, node)
}

type someResource struct {
	resource.Resource
	closeCap  []interface{}
	shouldErr bool
}

func (s *someResource) Close(ctx context.Context) error {
	s.closeCap = append(s.closeCap, ctx.Value("foo"))
	if s.shouldErr {
		return errors.New("bad close")
	}
	return nil
}

type anotherResource struct {
	resource.Resource
	CloseFunc func(ctx context.Context) error
}

// Close calls the injected Close or the real version.
func (a *anotherResource) Close(ctx context.Context) error {
	if a.CloseFunc == nil {
		return errors.New("oops")
	}
	return a.CloseFunc(ctx)
}

func TestClose(t *testing.T) {
	// Tests that Close does not deadlock by calling graphNode.Close() inside a resource Close().
	ourRes := &anotherResource{}
	node := resource.NewConfiguredGraphNode(
		resource.Config{},
		ourRes,
		resource.DefaultModelFamily.WithModel("bar"),
	)

	ourRes.CloseFunc = func(ctx context.Context) error {
		return node.Close(ctx)
	}

	// This pattern fails the test faster on deadlocks instead of having to wait for the full
	// test timeout.
	errCh := make(chan error)
	go func() {
		errCh <- node.Close(context.Background())
	}()

	select {
	case err := <-errCh:
		test.That(t, err, test.ShouldBeNil)
	case <-time.After(time.Second * 20):
		t.Fatal("node took too long to close, might be a deadlock")
	}
}

// TestTransitionToBlocking ensures a node marked removing cannot transition to [NodeStateUnhealthy] state.
func TestTransitionToBlocking(t *testing.T) {
	node := withTestLogger(t, resource.NewUninitializedNode())
	// Set state removing
	node.MarkForRemoval()
	test.That(t, node.MarkedForRemoval(), test.ShouldBeTrue)
	// Attempt to set state to [NodeStateUnhealthy]
	node.LogAndSetLastError(errors.New("Its error time"))
	// Node should stay still be in state removing
	test.That(t, node.MarkedForRemoval(), test.ShouldBeTrue)
}
