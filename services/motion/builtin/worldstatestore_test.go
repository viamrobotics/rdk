package builtin

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
	fakewss "go.viam.com/rdk/services/worldstatestore/fake"
	"go.viam.com/rdk/testutils/inject"
)

// boxTransform builds a world-state-store transform carrying a box obstacle at pose (x,y,z) in `parent`.
func boxTransform(uuid, parent string, x, y, z float64) *commonpb.Transform {
	return &commonpb.Transform{
		ReferenceFrame: uuid,
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: parent,
			Pose:           &commonpb.Pose{X: x, Y: y, Z: z, OZ: 1},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: 100, Y: 100, Z: 100}},
			},
		},
		Uuid: []byte(uuid),
	}
}

// storeFromTransforms builds an injected world state store backed by the given transforms.
func storeFromTransforms(name string, transforms ...*commonpb.Transform) *inject.WorldStateStoreService {
	byUUID := make(map[string]*commonpb.Transform, len(transforms))
	order := make([][]byte, 0, len(transforms))
	for _, tf := range transforms {
		byUUID[string(tf.Uuid)] = tf
		order = append(order, tf.Uuid)
	}
	svc := inject.NewWorldStateStoreService(name)
	svc.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return order, nil
	}
	svc.GetTransformFunc = func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
		tf, ok := byUUID[string(uuid)]
		if !ok {
			return nil, errors.New("not found")
		}
		return tf, nil
	}
	return svc
}

func newTestBuiltIn(t *testing.T, stores ...worldstatestore.Service) *builtIn {
	t.Helper()
	return &builtIn{logger: logging.NewTestLogger(t), worldStateStores: stores}
}

// obstacleCentersByLabel flattens a WorldState's obstacles into world frame and returns center points keyed by label.
func obstacleCentersByLabel(t *testing.T, ws *referenceframe.WorldState) map[string][3]float64 {
	t.Helper()
	fs := referenceframe.NewEmptyFrameSystem("test")
	gif, err := ws.ObstaclesInWorldFrame(fs, referenceframe.FrameSystemInputs{})
	test.That(t, err, test.ShouldBeNil)
	out := make(map[string][3]float64)
	for _, g := range gif.Geometries() {
		p := g.Pose().Point()
		out[g.Label()] = [3]float64{p.X, p.Y, p.Z}
	}
	return out
}

func TestWorldStateFromStoresNoStores(t *testing.T) {
	ms := newTestBuiltIn(t)
	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldBeNil)
}

func TestWorldStateFromStoresPlacesObstacles(t *testing.T) {
	store := storeFromTransforms("s1",
		boxTransform("box-a", "world", 100, 0, 0),
		boxTransform("box-b", "world", 0, 200, 0),
	)
	ms := newTestBuiltIn(t, store)

	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldNotBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 2)
	test.That(t, centers["box-a"], test.ShouldResemble, [3]float64{100, 0, 0})
	test.That(t, centers["box-b"], test.ShouldResemble, [3]float64{0, 200, 0})
}

func TestWorldStateFromStoresUnionAndDedup(t *testing.T) {
	store1 := storeFromTransforms("s1", boxTransform("box-a", "world", 100, 0, 0))
	// store2 repeats box-a (should be deduped, first store wins) and adds box-c.
	store2 := storeFromTransforms("s2",
		boxTransform("box-a", "world", 999, 999, 999),
		boxTransform("box-c", "world", 0, 0, 300),
	)
	ms := newTestBuiltIn(t, store1, store2)

	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 2)
	test.That(t, centers["box-a"], test.ShouldResemble, [3]float64{100, 0, 0}) // first store won
	test.That(t, centers["box-c"], test.ShouldResemble, [3]float64{0, 0, 300})
}

func TestWorldStateFromStoresGracefulFailure(t *testing.T) {
	bad := inject.NewWorldStateStoreService("bad")
	bad.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return nil, errors.New("boom")
	}
	good := storeFromTransforms("good", boxTransform("box-a", "world", 100, 0, 0))
	ms := newTestBuiltIn(t, bad, good)

	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldNotBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 1)
	test.That(t, centers["box-a"], test.ShouldResemble, [3]float64{100, 0, 0})
}

// TestWorldStateFromStoresWithRealFake exercises the end-to-end producer->consumer path: a real fake
// world state store in depth_camera mode feeds obstacles into the motion service's WorldState.
func TestWorldStateFromStoresWithRealFake(t *testing.T) {
	logger := logging.NewTestLogger(t)
	reg, ok := resource.LookupRegistration(worldstatestore.API, resource.DefaultModelFamily.WithModel("fake"))
	test.That(t, ok, test.ShouldBeTrue)

	conf := resource.Config{
		Name:                "dc",
		API:                 worldstatestore.API,
		Model:               resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fakewss.Config{InputSensorType: "depth_camera"},
	}
	res, err := reg.Constructor(context.Background(), nil, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	store := res.(worldstatestore.Service)
	defer store.Close(context.Background())

	ms := newTestBuiltIn(t, store)
	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldNotBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 3)
	test.That(t, centers, test.ShouldContainKey, "red-block")
	test.That(t, centers, test.ShouldContainKey, "green-block")
	test.That(t, centers, test.ShouldContainKey, "blue-block")
	// The table height (Z) is stable; X/Y carry simulated detection jitter.
	test.That(t, centers["red-block"][2], test.ShouldEqual, 25)
}

func TestWorldStateFromStoresGeometrylessBecomesTransform(t *testing.T) {
	noGeom := &commonpb.Transform{
		ReferenceFrame: "marker",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose:           &commonpb.Pose{X: 10, Y: 0, Z: 0, OZ: 1},
		},
		Uuid: []byte("marker"),
	}
	store := storeFromTransforms("s1", noGeom)
	ms := newTestBuiltIn(t, store)

	ws, err := ms.worldStateFromStores(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldNotBeNil)
	test.That(t, len(ws.Obstacles()), test.ShouldEqual, 0)
	test.That(t, len(ws.Transforms()), test.ShouldEqual, 1)
	test.That(t, ws.Transforms()[0].Name(), test.ShouldEqual, "marker")
}
