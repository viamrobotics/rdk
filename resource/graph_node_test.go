package resource_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils"
)

func TestUninitializedLifecycle(t *testing.T) {
	// empty
	node := resource.NewUninitializedNode()
	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	test.That(t, node.UpdatedAt(), test.ShouldEqual, 0)
	ok, _ := node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)
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

	lifecycleTest(t, node, []string(nil))
}

func TestUnconfiguredLifecycle(t *testing.T) {
	someConf := resource.Config{Attributes: utils.AttributeMap{"3": 4}}
	initialDeps := []string{"dep1", "dep2"}
	node := resource.NewUnconfiguredGraphNode(someConf, initialDeps)

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
	ok, _ := node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, initialDeps)

	lifecycleTest(t, node, initialDeps)
}

func TestConfiguredLifecycle(t *testing.T) {
	someConf := resource.Config{Attributes: utils.AttributeMap{"3": 4}}

	ourRes := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("some"))}
	node := resource.NewConfiguredGraphNode(someConf, ourRes, resource.DefaultModelFamily.WithModel("bar"))

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
	ok, _ := node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, node.UnresolvedDependencies(), test.ShouldBeEmpty)

	lifecycleTest(t, node, []string(nil))
}

func lifecycleTest(t *testing.T, node *resource.GraphNode, initialDeps []string) {
	// mark it for removal
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	node.MarkForRemoval()
	test.That(t, node.MarkedForRemoval(), test.ShouldBeTrue)

	_, err := node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "pending removal")

	ourErr := errors.New("whoops")
	node.SetLastError(ourErr)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "pending removal")

	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, initialDeps)

	// but we end up configuring it
	ourRes := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foo"))}
	node.SwapResource(ourRes, resource.DefaultModelFamily.WithModel("bar"))
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bar"))
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)

	res, err := node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)

	// now it needs update
	node.SetNeedsUpdate()
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	ok, _ := node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)

	// but an error happened
	node.IncrementTimesReconfigured()
	node.SetLastError(ourErr)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, ourErr.Error())
	res, err = node.UnsafeResource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)

	// it reconfigured
	ourRes2 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foo"))}
	node.SwapResource(ourRes2, resource.DefaultModelFamily.WithModel("baz"))
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("baz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)

	// it needs a new config
	ourConf := resource.Config{Attributes: utils.AttributeMap{"1": 2}}
	node.SetNewConfig(ourConf, []string{"3", "4", "5"})
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, []string{"3", "4", "5"})
	node.SetNeedsUpdate() // noop
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes2)
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldResemble, []string{"3", "4", "5"})

	// an error happens 5 (maxReconfigAttempts) times
	for i := 0; i < 5; i++ {
		ok, _ = node.CanReconfigure()
		test.That(t, ok, test.ShouldBeTrue)
		node.IncrementTimesReconfigured()
		node.SetLastError(ourErr)
		_, err = node.Resource()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ourErr.Error())
		res, err = node.UnsafeResource()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res, test.ShouldEqual, ourRes2)
		test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	}
	ok, err = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, err.Error(), test.ShouldContainSubstring, "Reconfiguration error")

	// retry with new config
	ourConf = resource.Config{Attributes: utils.AttributeMap{"1": 2}}
	node.SetNewConfig(ourConf, []string{"6", "7", "8"})
	test.That(t, node.NeedsReconfigure(), test.ShouldBeTrue)
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue) // test that SetNewConfig resets timesReconfigured
	ourRes3 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foo"))}
	node.SwapResource(ourRes3, resource.DefaultModelFamily.WithModel("bazz"))
	test.That(t, node.NeedsReconfigure(), test.ShouldBeFalse)
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue)
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldEqual, ourRes3)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})

	// but maxReconfigAttempts errors happen
	for i := 0; i < 5; i++ {
		ok, _ = node.CanReconfigure()
		test.That(t, ok, test.ShouldBeTrue)
		node.IncrementTimesReconfigured()
		node.SetLastError(ourErr)
		_, err = node.Resource()
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ourErr.Error())
		res, err = node.UnsafeResource()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, res, test.ShouldEqual, ourRes3)
	}
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeFalse)

	// set needs update is called
	node.SetNeedsUpdate()
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeFalse) // test that SetNeedsUpdate does not reset timesReconfigured

	// it finally reconfigured
	ourRes4 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("fooa"))}
	node.SwapResource(ourRes4, resource.DefaultModelFamily.WithModel("bazzz"))
	ok, _ = node.CanReconfigure()
	test.That(t, ok, test.ShouldBeTrue) // test SwapResource resets timesReconfigured
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bazzz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes3)
	test.That(t, res, test.ShouldEqual, ourRes4)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)
	test.That(t, node.Config(), test.ShouldResemble, resource.Config{Attributes: utils.AttributeMap{"1": 2}})
	test.That(t, node.UnresolvedDependencies(), test.ShouldBeEmpty)

	//nolint
	test.That(t, node.Close(context.WithValue(context.Background(), "foo", "hi")), test.ShouldBeNil)
	test.That(t, ourRes.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes3.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes4.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes4.closeCap, test.ShouldResemble, []interface{}{"hi"})

	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")

	ourRes5 := &someResource{Resource: testutils.NewUnimplementedResource(generic.Named("foob")), shoudlErr: true}
	node.SwapResource(ourRes5, resource.DefaultModelFamily.WithModel("bazzzz"))
	test.That(t, node.ResourceModel(), test.ShouldResemble, resource.DefaultModelFamily.WithModel("bazzzz"))
	res, err = node.Resource()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotEqual, ourRes4)
	test.That(t, res, test.ShouldEqual, ourRes5)
	test.That(t, node.MarkedForRemoval(), test.ShouldBeFalse)
	test.That(t, node.IsUninitialized(), test.ShouldBeFalse)

	//nolint
	err = node.Close(context.WithValue(context.Background(), "foo", "bye"))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "bad close")
	test.That(t, ourRes.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes2.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes3.closeCap, test.ShouldBeEmpty)
	test.That(t, ourRes4.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes5.closeCap, test.ShouldHaveLength, 1)
	test.That(t, ourRes5.closeCap, test.ShouldResemble, []interface{}{"bye"})

	test.That(t, node.IsUninitialized(), test.ShouldBeTrue)
	_, err = node.Resource()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not initialized")
}

type someResource struct {
	resource.Resource
	closeCap  []interface{}
	shoudlErr bool
}

func (s *someResource) Close(ctx context.Context) error {
	s.closeCap = append(s.closeCap, ctx.Value("foo"))
	if s.shoudlErr {
		return errors.New("bad close")
	}
	return nil
}
