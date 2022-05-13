package discovery_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

var (
	buttonSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("button"))
	button1       = resource.NameFromSubtype(buttonSubtype, "button1")
	button1Key    = discovery.Key{SubtypeName: button1.Subtype.ResourceSubtype, Model: "button1Model"}
	button2       = resource.NameFromSubtype(buttonSubtype, "button2")
	button2Key    = discovery.Key{SubtypeName: button2.Subtype.ResourceSubtype, Model: "button2Model"}
	button3       = resource.NameFromSubtype(buttonSubtype, "button3")
	button3Key    = discovery.Key{SubtypeName: button3.Subtype.ResourceSubtype, Model: "button3Model"}

	workingSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	workingModel   = "workingModel"
	working1       = resource.NameFromSubtype(workingSubtype, "working1")
	working1Key    = discovery.Key{workingSubtype.ResourceSubtype, workingModel}

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	failModel   = "failModel"
	fail1       = resource.NameFromSubtype(failSubtype, "fail1")
	fail1Key    = discovery.Key{failSubtype.ResourceSubtype, failModel}

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")

	mockReconfigurable = func(resource interface{}) (resource.Reconfigurable, error) {
		return nil, nil
	}
)

func init() {
	registry.RegisterResourceSubtype(
		workingSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	discovery.RegisterDiscoveryFunction(
		workingSubtype.ResourceSubtype,
		workingModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return workingDiscovery, nil
		},
	)

	registry.RegisterResourceSubtype(
		failSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	discovery.RegisterDiscoveryFunction(
		failSubtype.ResourceSubtype,
		failModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return nil, errFailed
		},
	)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	t.Run("found discovery service", func(t *testing.T) {
		svc, err := discovery.FromRobot(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)

		test.That(t, svc1.discoveryCount, test.ShouldEqual, 0)
		result, err := svc.Discover(context.Background(), []discovery.Key{button1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, discoveries)
		test.That(t, svc1.discoveryCount, test.ShouldEqual, 1)
	})

	t.Run("not discovery service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return "not discovery", nil
		}

		svc, err := discovery.FromRobot(r)
		test.That(t, err, test.ShouldBeError, utils.NewUnimplementedInterfaceError("discovery.Service", "string"))
		test.That(t, svc, test.ShouldBeNil)
	})

	t.Run("no discovery service", func(t *testing.T) {
		r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
			return nil, utils.NewResourceNotFoundError(discovery.Name)
		}

		svc, err := discovery.FromRobot(r)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(discovery.Name))
		test.That(t, svc, test.ShouldBeNil)
	})
}

func TestNew(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("no error", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)
	})
}

func TestDiscovery(t *testing.T) {
	logger := golog.NewTestLogger(t)
	discoveryKeys := []discovery.Key{working1Key, button1Key, fail1Key}
	resourceMap := map[resource.Name]interface{}{working1: "resource", button1: "resource", fail1: "resource"}

	t.Run("not found", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), []discovery.Key{button2Key})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))
	})

	t.Run("no CreateDiscovery", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.Discover(context.Background(), []discovery.Key{button1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, discoveries)
	})

	t.Run("failing resource", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), []discovery.Key{fail1Key})
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get discovery from %q", fail1))
	})

	t.Run("many discovery", func(t *testing.T) {
		// expected := map[resource.Name]interface{}{
		// 	working1: workingDiscovery,
		// 	button1:  struct{}{},
		// }
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), []discovery.Key{button2Key})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))

		resp, err := svc.Discover(context.Background(), []discovery.Key{working1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		s := resp[0]
		test.That(t, s.Key, test.ShouldResemble, working1Key)
		test.That(t, s.Discovered, test.ShouldResemble, workingDiscovery)

		resp, err = svc.Discover(context.Background(), []discovery.Key{working1Key, working1Key, working1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		s = resp[0]
		test.That(t, s.Key, test.ShouldResemble, working1Key)
		test.That(t, s.Discovered, test.ShouldResemble, workingDiscovery)

		// resp, err = svc.Discover(context.Background(), []discovery.Key{working1Key, button1Key})
		// test.That(t, err, test.ShouldBeNil)
		// test.That(t, len(resp), test.ShouldEqual, 2)
		// test.That(t, resp[0].Discovered, test.ShouldResemble, expected[resp[0].Key])
		// test.That(t, resp[1].Discovered, test.ShouldResemble, expected[resp[1].Key])

		_, err = svc.Discover(context.Background(), discoveryKeys)
		test.That(t, err, test.ShouldBeError, errors.Wrapf(errFailed, "failed to get discovery from %q", fail1))
	})
}

func TestUpdate(t *testing.T) {
	logger := golog.NewTestLogger(t)

	discoveryKeys := []discovery.Key{button1Key, button2Key}
	resourceMap := map[resource.Name]interface{}{button1: "resource", button2: "resource"}

	t.Run("update with no resources", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.Discover(context.Background(), []discovery.Key{button1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldResemble, discoveries)

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), []discovery.Key{button1Key})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button1))
	})

	t.Run("update with one resource", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.Discover(context.Background(), discoveryKeys)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Discovered, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Discovered, test.ShouldResemble, struct{}{})

		err = svc.(resource.Updateable).Update(context.Background(), map[resource.Name]interface{}{button1: "resource"})
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), discoveryKeys)
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button2))

		resp, err = svc.Discover(context.Background(), []discovery.Key{button1Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp, test.ShouldResemble, discoveries)
	})

	t.Run("update with same resources", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)
		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.Discover(context.Background(), discoveryKeys)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Discovered, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Discovered, test.ShouldResemble, struct{}{})

		err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
		test.That(t, err, test.ShouldBeNil)

		resp, err = svc.Discover(context.Background(), discoveryKeys)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 2)
		test.That(t, resp[0].Discovered, test.ShouldResemble, struct{}{})
		test.That(t, resp[1].Discovered, test.ShouldResemble, struct{}{})
	})

	t.Run("update with diff resources", func(t *testing.T) {
		svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		_, err = svc.Discover(context.Background(), []discovery.Key{button3Key})
		test.That(t, err, test.ShouldBeError, utils.NewResourceNotFoundError(button3))

		err = svc.(resource.Updateable).Update(
			context.Background(),
			map[resource.Name]interface{}{button3: "resource"},
		)
		test.That(t, err, test.ShouldBeNil)

		resp, err := svc.Discover(context.Background(), []discovery.Key{button3Key})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Key, test.ShouldResemble, button3Key)
		test.That(t, resp[0].Discovered, test.ShouldResemble, struct{}{})
	})
}

var discoveries = []discovery.Discovery{{Key: button1Key, Discovered: struct{}{}}}

type mock struct {
	discovery.Service

	discoveryCount int
}

func (m *mock) Discovery(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
	m.discoveryCount++
	return discoveries, nil
}
