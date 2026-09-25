package builtin

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
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

// builtInWithFrames returns a builtIn whose frame system reports the given frame names (plus world),
// so store-frame validation can be exercised.
func builtInWithFrames(t *testing.T, store worldstatestore.Service, frames ...string) *builtIn {
	t.Helper()
	parts := make([]*referenceframe.FrameSystemPart, 0, len(frames))
	for _, f := range frames {
		parts = append(parts, &referenceframe.FrameSystemPart{
			FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), f, nil),
		})
	}
	fsSvc := &inject.FrameSystemService{}
	fsSvc.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: parts}, nil
	}
	return &builtIn{logger: logging.NewTestLogger(t), worldStateStore: store, fsService: fsSvc}
}

// wsFromParts assembles a WorldState from raw parts for assertion convenience.
func wsFromParts(t *testing.T, obs []*referenceframe.GeometriesInFrame, tf []*referenceframe.LinkInFrame) *referenceframe.WorldState {
	t.Helper()
	ws, err := referenceframe.NewWorldState(obs, tf)
	test.That(t, err, test.ShouldBeNil)
	return ws
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

func TestStoreWorldStatePartsNoStore(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	obs, tf := ms.storeWorldStateParts(context.Background())
	test.That(t, obs, test.ShouldBeNil)
	test.That(t, tf, test.ShouldBeNil)
}

func TestStoreWorldStatePartsPlacesObstacles(t *testing.T) {
	store := storeFromTransforms("s1",
		boxTransform("box-a", "world", 100, 0, 0),
		boxTransform("box-b", "world", 0, 200, 0),
	)
	ms := newTestBuiltIn(t, store)

	obs, tf := ms.storeWorldStateParts(context.Background())
	centers := obstacleCentersByLabel(t, wsFromParts(t, obs, tf))
	test.That(t, len(centers), test.ShouldEqual, 2)
	test.That(t, centers["box-a"], test.ShouldResemble, [3]float64{100, 0, 0})
	test.That(t, centers["box-b"], test.ShouldResemble, [3]float64{0, 200, 0})
}

func TestStoreWorldStatePartsGracefulFailure(t *testing.T) {
	bad := inject.NewWorldStateStoreService("bad")
	bad.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return nil, errors.New("boom")
	}
	ms := newTestBuiltIn(t, bad)
	obs, tf := ms.storeWorldStateParts(context.Background())
	test.That(t, obs, test.ShouldBeNil)
	test.That(t, tf, test.ShouldBeNil)
}

func TestStoreWorldStatePartsGeometrylessBecomesTransform(t *testing.T) {
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

	obs, tf := ms.storeWorldStateParts(context.Background())
	test.That(t, len(obs), test.ShouldEqual, 0)
	test.That(t, len(tf), test.ShouldEqual, 1)
	test.That(t, tf[0].Name(), test.ShouldEqual, "marker")
}

// TestStoreWorldStatePartsWithRealFake exercises the end-to-end producer->consumer path: a real fake
// world state store in depth_camera mode feeds obstacles into the motion service's WorldState.
func TestStoreWorldStatePartsWithRealFake(t *testing.T) {
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
	obs, tf := ms.storeWorldStateParts(context.Background())
	centers := obstacleCentersByLabel(t, wsFromParts(t, obs, tf))
	test.That(t, len(centers), test.ShouldEqual, 3)
	test.That(t, centers, test.ShouldContainKey, "red-block")
	test.That(t, centers, test.ShouldContainKey, "green-block")
	test.That(t, centers, test.ShouldContainKey, "blue-block")
	test.That(t, centers["red-block"][2], test.ShouldEqual, 75) // rests on the floor at half the block height
}

// TestStoreWorldStatePartsLidarPointClouds verifies the lidar sim's point-cloud returns survive the
// producer->consumer round-trip as collision-capable obstacle geometries (octrees), not just transforms.
func TestStoreWorldStatePartsLidarPointClouds(t *testing.T) {
	logger := logging.NewTestLogger(t)
	reg, ok := resource.LookupRegistration(worldstatestore.API, resource.DefaultModelFamily.WithModel("fake"))
	test.That(t, ok, test.ShouldBeTrue)

	conf := resource.Config{
		Name:                "ld",
		API:                 worldstatestore.API,
		Model:               resource.DefaultModelFamily.WithModel("fake"),
		ConvertedAttributes: &fakewss.Config{InputSensorType: "lidar"},
	}
	res, err := reg.Constructor(context.Background(), nil, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	store := res.(worldstatestore.Service)
	defer store.Close(context.Background())

	ms := newTestBuiltIn(t, store)
	obs, tf := ms.storeWorldStateParts(context.Background())
	// Every return carries geometry, so all become obstacles (none degrade to geometry-less transforms).
	test.That(t, len(tf), test.ShouldEqual, 0)
	test.That(t, len(obs), test.ShouldBeGreaterThan, 0)
	for _, gif := range obs {
		for _, g := range gif.Geometries() {
			_, isOctree := g.(*pointcloud.BasicOctree)
			test.That(t, isOctree, test.ShouldBeTrue)
		}
	}
}

// TestStoreWorldStatePartsSkipsUnknownObstacleParent verifies an obstacle whose parent frame is not in
// the robot frame system is warned about and skipped (rather than failing the whole Move).
func TestStoreWorldStatePartsSkipsUnknownObstacleParent(t *testing.T) {
	store := storeFromTransforms("s1",
		boxTransform("known", "gripper", 0, 0, 0),
		boxTransform("orphan", "nonexistent", 0, 0, 0),
	)
	ms := builtInWithFrames(t, store, "gripper")

	obs, tf := ms.storeWorldStateParts(context.Background())
	names := wsFromParts(t, obs, tf).ObstacleNames()
	test.That(t, len(names), test.ShouldEqual, 1)
	test.That(t, names, test.ShouldContainKey, "known")
}

// TestStoreWorldStatePartsSkipsCollidingTransform verifies a geometry-less transform whose name collides
// with an existing frame is warned about and skipped (rather than blowing up NewFrameSystem).
func TestStoreWorldStatePartsSkipsCollidingTransform(t *testing.T) {
	frameless := func(name string) *commonpb.Transform {
		return &commonpb.Transform{
			ReferenceFrame:      name,
			PoseInObserverFrame: &commonpb.PoseInFrame{ReferenceFrame: "world", Pose: &commonpb.Pose{OZ: 1}},
			Uuid:                []byte(name),
		}
	}
	store := storeFromTransforms("s1", frameless("gripper"), frameless("marker"))
	ms := builtInWithFrames(t, store, "gripper")

	_, tf := ms.storeWorldStateParts(context.Background())
	test.That(t, len(tf), test.ShouldEqual, 1)
	test.That(t, tf[0].Name(), test.ShouldEqual, "marker")
}

// TestMoveWithWorldStateStore drives plan() with a store configured: a Move with no WorldState picks up
// the store's obstacles, and store entries referencing unknown frames are skipped rather than fatal.
func TestMoveWithWorldStateStore(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	moveReq := motion.MoveReq{
		ComponentName: "pieceGripper",
		Destination:   referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50})),
	}

	planWithStore := func(t *testing.T, transforms ...*commonpb.Transform) error {
		t.Helper()
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		ms.(*builtIn).worldStateStore = storeFromTransforms("s", transforms...)
		_, err := ms.(*builtIn).plan(ctx, moveReq, logger)
		return err
	}

	t.Run("obstacle in a known frame plans", func(t *testing.T) {
		test.That(t, planWithStore(t, boxTransform("obs", "world", 500, 500, 500)), test.ShouldBeNil)
	})

	// Without the frame-validation fix, an obstacle in an unknown frame errors ObstaclesInWorldFrame and
	// fails the Move; with it, the obstacle is skipped and planning proceeds.
	t.Run("obstacle in an unknown frame is skipped, not fatal", func(t *testing.T) {
		test.That(t, planWithStore(t, boxTransform("orphan", "nonexistent_frame", 0, 0, 0)), test.ShouldBeNil)
	})

	// Without the fix, a geometry-less transform colliding with an existing frame errors NewFrameSystem.
	t.Run("transform colliding with an existing frame is skipped, not fatal", func(t *testing.T) {
		collide := &commonpb.Transform{
			ReferenceFrame:      "pieceArm",
			PoseInObserverFrame: &commonpb.PoseInFrame{ReferenceFrame: "world", Pose: &commonpb.Pose{OZ: 1}},
			Uuid:                []byte("pieceArm"),
		}
		test.That(t, planWithStore(t, collide), test.ShouldBeNil)
	})
}

func TestMergeWorldStateNilCases(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	ctx := context.Background()

	test.That(t, ms.mergeWorldState(ctx, nil, nil, nil), test.ShouldBeNil)

	// No store parts: the request world state is returned untouched (same pointer).
	req := wsWithBoxes(t, "a")
	test.That(t, ms.mergeWorldState(ctx, req, nil, nil), test.ShouldEqual, req)

	// No request: a world state is built from the store parts.
	store := wsWithBoxes(t, "b")
	merged := ms.mergeWorldState(ctx, nil, store.Obstacles(), store.Transforms())
	test.That(t, merged, test.ShouldNotBeNil)
	test.That(t, merged.ObstacleNames(), test.ShouldContainKey, "b")
}

func TestMergeWorldStateCombinesObstacles(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	req := wsWithBoxes(t, "req-a")
	store := wsWithBoxes(t, "store-a", "store-b")

	merged := ms.mergeWorldState(context.Background(), req, store.Obstacles(), store.Transforms())
	names := merged.ObstacleNames()
	test.That(t, len(names), test.ShouldEqual, 3)
	test.That(t, names, test.ShouldContainKey, "req-a")
	test.That(t, names, test.ShouldContainKey, "store-a")
	test.That(t, names, test.ShouldContainKey, "store-b")
}

// TestMergeWorldStateCollisionKeepsBoth verifies a name present in both sources is not dropped: both are
// kept and the store copy is renamed.
func TestMergeWorldStateCollisionKeepsBoth(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	req := wsWithBoxes(t, "dup")
	store := wsWithBoxes(t, "dup")

	merged := ms.mergeWorldState(context.Background(), req, store.Obstacles(), store.Transforms())
	names := merged.ObstacleNames()
	test.That(t, len(names), test.ShouldEqual, 2)
	test.That(t, names, test.ShouldContainKey, "dup")
	test.That(t, names, test.ShouldContainKey, "dup#2")
}

// TestMergeWorldStateTransformCollision verifies transform-name collisions are also disambiguated.
func TestMergeWorldStateTransformCollision(t *testing.T) {
	ms := newTestBuiltIn(t, nil)
	link := func(name string) *referenceframe.LinkInFrame {
		return referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), name, nil)
	}
	req, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{link("t1")})
	test.That(t, err, test.ShouldBeNil)
	store, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{link("t1")})
	test.That(t, err, test.ShouldBeNil)

	merged := ms.mergeWorldState(context.Background(), req, store.Obstacles(), store.Transforms())
	got := make([]string, 0, len(merged.Transforms()))
	for _, l := range merged.Transforms() {
		got = append(got, l.Name())
	}
	test.That(t, len(got), test.ShouldEqual, 2)
	test.That(t, got, test.ShouldContain, "t1")
	test.That(t, got, test.ShouldContain, "t1#2")
}
