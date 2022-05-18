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
	workingSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("working"))
	workingModel   = "workingModel"
	workingKey     = discovery.Key{workingSubtype.ResourceSubtype, workingModel}

	failSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("fail"))
	failModel   = "failModel"
	failKey     = discovery.Key{failSubtype.ResourceSubtype, failModel}

	noDiscoverSubtype = resource.NewSubtype(resource.Namespace("acme"), resource.ResourceTypeComponent, resource.SubtypeName("nodiscover"))
	noDiscoverModel   = "nodiscover"
	noDiscoverKey     = discovery.Key{failSubtype.ResourceSubtype, noDiscoverModel}

	missingKey = discovery.Key{failSubtype.ResourceSubtype, "missing"}

	workingDiscovery = map[string]interface{}{"position": "up"}
	errFailed        = errors.New("can't get discovery")

	mockReconfigurable = func(resource interface{}) (resource.Reconfigurable, error) {
		return nil, nil
	}
)

func init() {
	// Subtype with a working discovery function for a subtype model
	registry.RegisterResourceSubtype(
		workingSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	discovery.RegisterFunction(
		workingSubtype.ResourceSubtype,
		workingModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return workingDiscovery, nil
		},
	)

	// Subtype without discovery function
	registry.RegisterResourceSubtype(
		noDiscoverSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	// Subtype with a failing discovery function for a subtype model
	registry.RegisterResourceSubtype(
		failSubtype,
		registry.ResourceSubtype{Reconfigurable: mockReconfigurable},
	)

	discovery.RegisterFunction(
		failSubtype.ResourceSubtype,
		failModel,
		func(ctx context.Context, subtypeName resource.SubtypeName, model string) (interface{}, error) {
			return nil, errFailed
		},
	)
}

var discoveries = []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}}

type mock struct {
	discovery.Discovery

	discoveryCount int
}

func (m *mock) Discover(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
	m.discoveryCount++
	return discoveries, nil
}

// func setupInjectRobot(t *testing.T) (*inject.Robot, *mock) {
// 	t.Helper()
// 	svc1 := &mock{}
// 	r := &inject.Robot{}
// 	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
// 		return svc1, nil
// 	}
// 	return r, svc1
// }

func TestFromRobot(t *testing.T) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}

	t.Run("found discovery service", func(t *testing.T) {
		svc, err := discovery.FromRobot(r)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, svc, test.ShouldNotBeNil)

		test.That(t, svc1.discoveryCount, test.ShouldEqual, 0)
		result, err := svc.Discover(context.Background(), []discovery.Key{workingKey})
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

func setupNewDiscoveryService(t *testing.T) discovery.Service {
	t.Helper()

	logger := golog.NewTestLogger(t)
	resourceMap := map[resource.Name]interface{}{}

	svc, err := discovery.New(context.Background(), &inject.Robot{}, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	err = svc.(resource.Updateable).Update(context.Background(), resourceMap)
	test.That(t, err, test.ShouldBeNil)

	return svc
}

func TestDiscovery(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)
		discoveries, err := svc.Discover(context.Background(), []discovery.Key{missingKey})
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("no Discover", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)

		discoveries, err := svc.Discover(context.Background(), []discovery.Key{noDiscoverKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldBeEmpty)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("failing Discover", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)

		_, err := svc.Discover(context.Background(), []discovery.Key{failKey})
		test.That(t, err, test.ShouldBeError, &discovery.DiscoverError{failKey})
	})

	t.Run("working Discover", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)

		discoveries, err := svc.Discover(context.Background(), []discovery.Key{workingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})

	t.Run("duplicated working Discover", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)

		discoveries, err := svc.Discover(context.Background(), []discovery.Key{workingKey, workingKey, workingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})

	t.Run("working and missing Discover", func(t *testing.T) {
		svc := setupNewDiscoveryService(t)

		discoveries, err := svc.Discover(context.Background(), []discovery.Key{workingKey, missingKey})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, discoveries, test.ShouldResemble, []discovery.Discovery{{Key: workingKey, Discovered: workingDiscovery}})
	})
}
