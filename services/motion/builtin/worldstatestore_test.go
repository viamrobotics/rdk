package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
	fakewss "go.viam.com/rdk/services/worldstatestore/fake"
	"go.viam.com/rdk/spatialmath"
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

func newTestBuiltIn(t *testing.T, store worldstatestore.Service) *builtIn {
	t.Helper()
	return &builtIn{logger: logging.NewTestLogger(t), worldStateStore: store}
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

// wsWithBoxes builds a WorldState containing a labeled box obstacle (in the world frame) per name.
func wsWithBoxes(t *testing.T, names ...string) *referenceframe.WorldState {
	t.Helper()
	obstacles := make([]*referenceframe.GeometriesInFrame, 0, len(names))
	for i, name := range names {
		box, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{X: float64(i * 100)}),
			r3.Vector{X: 50, Y: 50, Z: 50}, name,
		)
		test.That(t, err, test.ShouldBeNil)
		obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{box}))
	}
	ws, err := referenceframe.NewWorldState(obstacles, nil)
	test.That(t, err, test.ShouldBeNil)
	return ws
}

func TestWorldStateFromStoreNoStore(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	test.That(t, ms.worldStateFromStore(context.Background()), test.ShouldBeNil)
}

func TestWorldStateFromStorePlacesObstacles(t *testing.T) {
	store := storeFromTransforms("s1",
		boxTransform("box-a", "world", 100, 0, 0),
		boxTransform("box-b", "world", 0, 200, 0),
	)
	ms := newTestBuiltIn(t, store)

	ws := ms.worldStateFromStore(context.Background())
	test.That(t, ws, test.ShouldNotBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 2)
	test.That(t, centers["box-a"], test.ShouldResemble, [3]float64{100, 0, 0})
	test.That(t, centers["box-b"], test.ShouldResemble, [3]float64{0, 200, 0})
}

func TestWorldStateFromStoreGracefulFailure(t *testing.T) {
	bad := inject.NewWorldStateStoreService("bad")
	bad.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return nil, errors.New("boom")
	}
	ms := newTestBuiltIn(t, bad)
	test.That(t, ms.worldStateFromStore(context.Background()), test.ShouldBeNil)
}

func TestWorldStateFromStoreGeometrylessBecomesTransform(t *testing.T) {
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

	ws := ms.worldStateFromStore(context.Background())
	test.That(t, ws, test.ShouldNotBeNil)
	test.That(t, len(ws.Obstacles()), test.ShouldEqual, 0)
	test.That(t, len(ws.Transforms()), test.ShouldEqual, 1)
	test.That(t, ws.Transforms()[0].Name(), test.ShouldEqual, "marker")
}

// TestWorldStateFromStoreWithRealFake exercises the end-to-end producer->consumer path: a real fake
// world state store in depth_camera mode feeds obstacles into the motion service's WorldState.
func TestWorldStateFromStoreWithRealFake(t *testing.T) {
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
	ws := ms.worldStateFromStore(context.Background())
	test.That(t, ws, test.ShouldNotBeNil)

	centers := obstacleCentersByLabel(t, ws)
	test.That(t, len(centers), test.ShouldEqual, 3)
	test.That(t, centers, test.ShouldContainKey, "red-block")
	test.That(t, centers, test.ShouldContainKey, "green-block")
	test.That(t, centers, test.ShouldContainKey, "blue-block")
	test.That(t, centers["red-block"][2], test.ShouldEqual, 25) // stable table height
}

func TestMergeWorldStatesNilCases(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	ctx := context.Background()

	test.That(t, ms.mergeWorldStates(ctx, nil, nil), test.ShouldBeNil)

	req := wsWithBoxes(t, "a")
	test.That(t, ms.mergeWorldStates(ctx, req, nil), test.ShouldEqual, req)

	store := wsWithBoxes(t, "b")
	test.That(t, ms.mergeWorldStates(ctx, nil, store), test.ShouldEqual, store)
}

func TestMergeWorldStatesCombinesObstacles(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	req := wsWithBoxes(t, "req-a")
	store := wsWithBoxes(t, "store-a", "store-b")

	merged := ms.mergeWorldStates(context.Background(), req, store)
	names := merged.ObstacleNames()
	test.That(t, len(names), test.ShouldEqual, 3)
	test.That(t, names, test.ShouldContainKey, "req-a")
	test.That(t, names, test.ShouldContainKey, "store-a")
	test.That(t, names, test.ShouldContainKey, "store-b")
}

// TestMergeWorldStatesCollisionKeepsBoth verifies a name present in both sources is not dropped: both are
// kept and the store copy is renamed.
func TestMergeWorldStatesCollisionKeepsBoth(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	req := wsWithBoxes(t, "dup")
	store := wsWithBoxes(t, "dup")

	merged := ms.mergeWorldStates(context.Background(), req, store)
	names := merged.ObstacleNames()
	test.That(t, len(names), test.ShouldEqual, 2)
	test.That(t, names, test.ShouldContainKey, "dup")
	test.That(t, names, test.ShouldContainKey, "dup#2")
}

// TestMergeWorldStatesTransformCollision verifies transform-name collisions are also disambiguated.
func TestMergeWorldStatesTransformCollision(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	link := func(name string) *referenceframe.LinkInFrame {
		return referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), name, nil)
	}
	req, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{link("t1")})
	test.That(t, err, test.ShouldBeNil)
	store, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{link("t1")})
	test.That(t, err, test.ShouldBeNil)

	merged := ms.mergeWorldStates(context.Background(), req, store)
	got := make([]string, 0, len(merged.Transforms()))
	for _, l := range merged.Transforms() {
		got = append(got, l.Name())
	}
	test.That(t, len(got), test.ShouldEqual, 2)
	test.That(t, got, test.ShouldContain, "t1")
	test.That(t, got, test.ShouldContain, "t1#2")
}
