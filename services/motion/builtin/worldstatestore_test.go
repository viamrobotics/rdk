package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/worldstatestore"
	wssbuiltin "go.viam.com/rdk/services/worldstatestore/builtin"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

// storeTransform builds a transform for a world state store. With a non-zero `boxSize` it carries
// a cube of that size, which makes it an obstacle.
func storeTransform(name, parent string, center r3.Vector, boxSize float64) *commonpb.Transform {
	ret := &commonpb.Transform{
		Uuid:           []byte(name),
		ReferenceFrame: name,
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: parent,
			Pose:           &commonpb.Pose{X: center.X, Y: center.Y, Z: center.Z, OZ: 1},
		},
	}
	if boxSize != 0 {
		ret.PhysicalObject = &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: boxSize, Y: boxSize, Z: boxSize}},
			},
		}
	}
	return ret
}

// newStore returns a builtin world state store holding the given transforms, added the way a
// client would add them.
func newStore(t *testing.T, transforms ...*commonpb.Transform) worldstatestore.Service {
	t.Helper()
	return newNamedStore(t, "store", transforms...)
}

// withMetadata returns the transform with its metadata set.
func withMetadata(t *testing.T, transform *commonpb.Transform, metadata map[string]interface{}) *commonpb.Transform {
	t.Helper()

	var err error
	transform.Metadata, err = structpb.NewStruct(metadata)
	test.That(t, err, test.ShouldBeNil)
	return transform
}

// visualOnly returns the transform marked as not to be planned around.
func visualOnly(t *testing.T, transform *commonpb.Transform) *commonpb.Transform {
	t.Helper()
	return withMetadata(t, transform, map[string]interface{}{worldstatestore.VisualOnlyMetadataKey: true})
}

func newNamedStore(t *testing.T, name string, transforms ...*commonpb.Transform) worldstatestore.Service {
	t.Helper()

	store, err := wssbuiltin.NewBuiltIn(worldstatestore.Named(name), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, store.Close(context.Background()), test.ShouldBeNil) })

	items := make([]interface{}, 0, len(transforms))
	for _, transform := range transforms {
		encoded, err := protojson.Marshal(transform)
		test.That(t, err, test.ShouldBeNil)
		var item map[string]interface{}
		test.That(t, json.Unmarshal(encoded, &item), test.ShouldBeNil)
		items = append(items, item)
	}
	if len(items) > 0 {
		_, err = store.DoCommand(context.Background(), map[string]interface{}{wssbuiltin.DoAdd: items})
		test.That(t, err, test.ShouldBeNil)
	}
	return store
}

// newMotionWithStore returns a motion service that only has what reading a store needs: the store
// and a frame system reporting the given frames, each a child of the world frame.
func newMotionWithStore(t *testing.T, store worldstatestore.Service, frames ...string) *builtIn {
	t.Helper()

	if store == nil {
		return newMotionWithStores(t, frames)
	}
	return newMotionWithStores(t, frames, store)
}

// newMotionWithStores is newMotionWithStore for any number of stores, in the order given.
func newMotionWithStores(t *testing.T, frames []string, stores ...worldstatestore.Service) *builtIn {
	t.Helper()

	parts := make([]*referenceframe.FrameSystemPart, 0, len(frames))
	for _, frame := range frames {
		parts = append(parts, &referenceframe.FrameSystemPart{
			FrameConfig: referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), frame, nil),
		})
	}
	fsService := &inject.FrameSystemService{}
	fsService.FrameSystemConfigFunc = func(ctx context.Context) (*framesystem.Config, error) {
		return &framesystem.Config{Parts: parts}, nil
	}
	return &builtIn{logger: logging.NewTestLogger(t), worldStateStores: stores, fsService: fsService}
}

// obstacleParents returns the parent frame of each obstacle, keyed by the obstacle's label.
func obstacleParents(obstacles []*referenceframe.GeometriesInFrame) map[string]string {
	ret := make(map[string]string)
	for _, inFrame := range obstacles {
		for _, geometry := range inFrame.Geometries() {
			ret[geometry.Label()] = inFrame.Parent()
		}
	}
	return ret
}

func transformNames(transforms []*referenceframe.LinkInFrame) []string {
	ret := make([]string, 0, len(transforms))
	for _, link := range transforms {
		ret = append(ret, link.Name())
	}
	return ret
}

// boxWorldState returns a world state holding one labeled box in the world frame per name.
func boxWorldState(t *testing.T, names ...string) *referenceframe.WorldState {
	t.Helper()

	obstacles := make([]*referenceframe.GeometriesInFrame, 0, len(names))
	for idx, name := range names {
		pose := spatialmath.NewPoseFromPoint(r3.Vector{X: float64(idx * 100)})
		box, err := spatialmath.NewBox(pose, r3.Vector{X: 50, Y: 50, Z: 50}, name)
		test.That(t, err, test.ShouldBeNil)
		obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{box}))
	}
	ret, err := referenceframe.NewWorldState(obstacles, nil)
	test.That(t, err, test.ShouldBeNil)
	return ret
}

// TestStoreWorldState asserts how the contents of a store become planner inputs: transforms with
// a geometry become obstacles placed at their pose, and the rest become supplemental frames.
func TestStoreWorldState(t *testing.T) {
	ctx := context.Background()

	store := newStore(t,
		storeTransform("box-in-world", referenceframe.World, r3.Vector{X: 100}, 10),
		storeTransform("box-on-gripper", "gripper", r3.Vector{Y: 200}, 10),
		// A supplemental frame, and an obstacle parented to it.
		storeTransform("marker", referenceframe.World, r3.Vector{Z: 300}, 0),
		storeTransform("box-on-marker", "marker", r3.Vector{}, 10),
	)
	ms := newMotionWithStore(t, store, "gripper")

	obstacles, transforms := ms.storeWorldState(ctx, nil)
	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"marker"})
	test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{
		"box-in-world":   referenceframe.World,
		"box-on-gripper": "gripper",
		"box-on-marker":  "marker",
	})

	// Each obstacle sits at its transform's pose within its parent frame.
	for _, inFrame := range obstacles {
		geometry := inFrame.Geometries()[0]
		switch geometry.Label() {
		case "box-in-world":
			test.That(t, geometry.Pose().Point(), test.ShouldResemble, r3.Vector{X: 100})
		case "box-on-gripper":
			test.That(t, geometry.Pose().Point(), test.ShouldResemble, r3.Vector{Y: 200})
		case "box-on-marker":
			test.That(t, geometry.Pose().Point(), test.ShouldResemble, r3.Vector{})
		}
	}

	// The result is a valid world state.
	_, err := referenceframe.NewWorldState(obstacles, transforms)
	test.That(t, err, test.ShouldBeNil)
}

// TestStoreWorldStateSkips asserts that store contents the planner cannot use are left out rather
// than failing the move.
func TestStoreWorldStateSkips(t *testing.T) {
	ctx := context.Background()

	t.Run("obstacle in a frame that does not exist", func(t *testing.T) {
		store := newStore(t,
			storeTransform("known", "gripper", r3.Vector{}, 10),
			storeTransform("orphan", "nonexistent", r3.Vector{}, 10),
		)
		obstacles, transforms := newMotionWithStore(t, store, "gripper").storeWorldState(ctx, nil)
		test.That(t, transforms, test.ShouldBeEmpty)
		test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{"known": "gripper"})
	})

	t.Run("supplemental frame named like an existing frame", func(t *testing.T) {
		store := newStore(t,
			storeTransform("gripper", referenceframe.World, r3.Vector{}, 0),
			storeTransform("marker", referenceframe.World, r3.Vector{}, 0),
		)
		obstacles, transforms := newMotionWithStore(t, store, "gripper").storeWorldState(ctx, nil)
		test.That(t, obstacles, test.ShouldBeEmpty)
		test.That(t, transformNames(transforms), test.ShouldResemble, []string{"marker"})
	})

	t.Run("store that cannot be listed", func(t *testing.T) {
		store := inject.NewWorldStateStoreService("broken")
		store.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
			return nil, errors.New("boom")
		}
		obstacles, transforms := newMotionWithStore(t, store).storeWorldState(ctx, nil)
		test.That(t, obstacles, test.ShouldBeEmpty)
		test.That(t, transforms, test.ShouldBeEmpty)
	})

	t.Run("transform that cannot be read", func(t *testing.T) {
		store := inject.NewWorldStateStoreService("flaky")
		store.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
			return [][]byte{[]byte("gone"), []byte("box")}, nil
		}
		store.GetTransformFunc = func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
			if string(uuid) == "gone" {
				return nil, errors.New("not found")
			}
			return storeTransform("box", referenceframe.World, r3.Vector{}, 10), nil
		}
		obstacles, _ := newMotionWithStore(t, store).storeWorldState(ctx, nil)
		test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{"box": referenceframe.World})
	})
}

// TestStoreWorldStateFrames asserts which supplemental frames a store contributes when they
// depend on each other or on frames that do not exist.
func TestStoreWorldStateFrames(t *testing.T) {
	ctx := context.Background()

	// "inner" is parented to "outer", and an obstacle to "inner". "lost" has no parent to attach
	// to, and neither does the obstacle that depends on it.
	store := newStore(t,
		storeTransform("box-on-inner", "inner", r3.Vector{}, 10),
		storeTransform("inner", "outer", r3.Vector{X: 1}, 0),
		storeTransform("outer", referenceframe.World, r3.Vector{X: 2}, 0),
		storeTransform("lost", "nowhere", r3.Vector{}, 0),
		storeTransform("box-on-lost", "lost", r3.Vector{}, 10),
	)
	obstacles, transforms := newMotionWithStore(t, store).storeWorldState(ctx, nil)

	// A frame is only accepted after its parent, whatever order the store listed them in.
	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"outer", "inner"})
	test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{"box-on-inner": "inner"})

	// What is left builds into a frame system, which a frame with a missing parent would not.
	frameSystem, err := referenceframe.NewFrameSystem("test", nil, transforms)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, frameSystem.Frame("inner"), test.ShouldNotBeNil)

	// A transform the request supplies defines its name. The store's frame of that name is
	// skipped, and the store's obstacle is parented to the request's.
	request, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), "inner", nil),
	})
	test.That(t, err, test.ShouldBeNil)
	obstacles, transforms = newMotionWithStore(t, store).storeWorldState(ctx, request)
	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"outer"})
	test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{"box-on-inner": "inner"})
}

// TestStoreWorldStateVisualOnly asserts that a transform marked visual only is not an obstacle,
// and that only a boolean true marks it so.
func TestStoreWorldStateVisualOnly(t *testing.T) {
	ctx := context.Background()

	store := newStore(t,
		storeTransform("obstacle", referenceframe.World, r3.Vector{}, 10),
		visualOnly(t, storeTransform("visual", referenceframe.World, r3.Vector{X: 100}, 10)),
		// Something parented to a visual-only transform still has a frame to attach to.
		storeTransform("box-on-visual", "visual", r3.Vector{}, 10),
		// None of these mark the transform, so each stays an obstacle.
		withMetadata(t, storeTransform("marked-false", referenceframe.World, r3.Vector{}, 10),
			map[string]interface{}{worldstatestore.VisualOnlyMetadataKey: false}),
		withMetadata(t, storeTransform("marked-with-a-string", referenceframe.World, r3.Vector{}, 10),
			map[string]interface{}{worldstatestore.VisualOnlyMetadataKey: "true"}),
		withMetadata(t, storeTransform("marked-with-a-number", referenceframe.World, r3.Vector{}, 10),
			map[string]interface{}{worldstatestore.VisualOnlyMetadataKey: 1}),
		withMetadata(t, storeTransform("other-metadata", referenceframe.World, r3.Vector{}, 10),
			map[string]interface{}{"color": "red"}),
	)
	obstacles, transforms := newMotionWithStore(t, store).storeWorldState(ctx, nil)

	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"visual"})
	test.That(t, transforms[0].Geometry(), test.ShouldBeNil)
	test.That(t, transforms[0].Pose().Point(), test.ShouldResemble, r3.Vector{X: 100})
	test.That(t, obstacleParents(obstacles), test.ShouldResemble, map[string]string{
		"obstacle":             referenceframe.World,
		"box-on-visual":        "visual",
		"marked-false":         referenceframe.World,
		"marked-with-a-string": referenceframe.World,
		"marked-with-a-number": referenceframe.World,
		"other-metadata":       referenceframe.World,
	})
}

// TestStoreWorldStateMultipleStores asserts that several stores are read together: each
// contributes, they can build on each other's frames, and one failing does not affect the rest.
func TestStoreWorldStateMultipleStores(t *testing.T) {
	ctx := context.Background()

	first := newNamedStore(t, "first",
		storeTransform("first-box", referenceframe.World, r3.Vector{X: 1}, 10),
		storeTransform("shared-frame", referenceframe.World, r3.Vector{X: 10}, 0),
		storeTransform("same-label", referenceframe.World, r3.Vector{}, 10),
	)
	broken := inject.NewWorldStateStoreService("broken")
	broken.ListUUIDsFunc = func(ctx context.Context, extra map[string]any) ([][]byte, error) {
		return nil, errors.New("boom")
	}
	second := newNamedStore(t, "second",
		// Parented to a frame that only the first store defines.
		storeTransform("second-box", "shared-frame", r3.Vector{X: 2}, 10),
		// The first store already defined this frame, at another pose.
		storeTransform("shared-frame", referenceframe.World, r3.Vector{X: 999}, 0),
		storeTransform("same-label", referenceframe.World, r3.Vector{}, 10),
	)
	ms := newMotionWithStores(t, nil, first, broken, second)

	obstacles, transforms := ms.storeWorldState(ctx, nil)

	// The first store's definition of the shared frame stands, and the second store's obstacle
	// attaches to it.
	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"shared-frame"})
	test.That(t, transforms[0].Pose().Point(), test.ShouldResemble, r3.Vector{X: 10})
	parents := make(map[string]string)
	labels := make([]string, 0, len(obstacles))
	for _, inFrame := range obstacles {
		label := inFrame.Geometries()[0].Label()
		labels = append(labels, label)
		parents[label] = inFrame.Parent()
	}
	test.That(t, parents["first-box"], test.ShouldEqual, referenceframe.World)
	test.That(t, parents["second-box"], test.ShouldEqual, "shared-frame")

	// Stores are read in the order they are configured, and the broken one contributes nothing.
	test.That(t, labels, test.ShouldHaveLength, 4)
	fromFirst, fromSecond := slices.Clone(labels[:2]), slices.Clone(labels[2:])
	slices.Sort(fromFirst)
	slices.Sort(fromSecond)
	test.That(t, fromFirst, test.ShouldResemble, []string{"first-box", "same-label"})
	test.That(t, fromSecond, test.ShouldResemble, []string{"same-label", "second-box"})

	// Merged into a world state, the label both stores use is kept for both obstacles.
	merged := ms.withStoreWorldState(ctx, boxWorldState(t, "request-box"))
	test.That(t, merged.ObstacleNames(), test.ShouldResemble, map[string]bool{
		"request-box":  true,
		"first-box":    true,
		"second-box":   true,
		"same-label":   true,
		"same-label#2": true,
	})
	test.That(t, transformNames(merged.Transforms()), test.ShouldResemble, []string{"shared-frame"})

	// Reading the other way round, the other store's definition stands.
	_, transforms = newMotionWithStores(t, nil, second, first).storeWorldState(ctx, nil)
	test.That(t, transformNames(transforms), test.ShouldResemble, []string{"shared-frame"})
	test.That(t, transforms[0].Pose().Point(), test.ShouldResemble, r3.Vector{X: 999})

	// No stores at all leaves the request's world state as it is.
	request := boxWorldState(t, "request-box")
	test.That(t, newMotionWithStores(t, nil).withStoreWorldState(ctx, request), test.ShouldEqual, request)
}

// TestMergeWorldState asserts that the request's world state and the store's are combined without
// dropping anything from either.
func TestMergeWorldState(t *testing.T) {
	ctx := context.Background()
	ms := newMotionWithStore(t, nil)

	// With nothing from the store, the request's world state is passed through untouched.
	test.That(t, ms.mergeWorldState(ctx, nil, nil, nil), test.ShouldBeNil)
	request := boxWorldState(t, "a")
	test.That(t, ms.mergeWorldState(ctx, request, nil, nil), test.ShouldEqual, request)

	// With no request world state, the result is the store's.
	fromTheStore := boxWorldState(t, "b")
	merged := ms.mergeWorldState(ctx, nil, fromTheStore.Obstacles(), nil)
	test.That(t, merged.ObstacleNames(), test.ShouldResemble, map[string]bool{"b": true})

	// With both, the result holds the obstacles of both.
	merged = ms.mergeWorldState(ctx, boxWorldState(t, "request-a"), boxWorldState(t, "store-a", "store-b").Obstacles(), nil)
	test.That(t, merged.ObstacleNames(), test.ShouldResemble,
		map[string]bool{"request-a": true, "store-a": true, "store-b": true})

	// An obstacle name used by both is kept twice, the store's renamed. The request's world state
	// is not modified.
	request = boxWorldState(t, "dup")
	merged = ms.mergeWorldState(ctx, request, boxWorldState(t, "dup").Obstacles(), nil)
	test.That(t, merged.ObstacleNames(), test.ShouldResemble, map[string]bool{"dup": true, "dup#2": true})
	test.That(t, request.ObstacleNames(), test.ShouldResemble, map[string]bool{"dup": true})

	// The same holds for transforms.
	link := func(name string) *referenceframe.LinkInFrame {
		return referenceframe.NewLinkInFrame(referenceframe.World, spatialmath.NewZeroPose(), name, nil)
	}
	request, err := referenceframe.NewWorldState(nil, []*referenceframe.LinkInFrame{link("frame")})
	test.That(t, err, test.ShouldBeNil)
	merged = ms.mergeWorldState(ctx, request, nil, []*referenceframe.LinkInFrame{link("frame")})
	test.That(t, transformNames(merged.Transforms()), test.ShouldResemble, []string{"frame", "frame#2"})
}

// TestWorldStateStoreConfig asserts that naming stores makes them dependencies, and that the
// motion service picks them out of its dependencies in the order the config names them.
func TestWorldStateStoreConfig(t *testing.T) {
	ctx := context.Background()

	// Without a store named, the only dependency is the frame system.
	deps, _, err := (&Config{}).Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{framesystem.InternalServiceName.String()})

	conf := &Config{WorldStateStores: []string{"second", "first"}}
	deps, _, err = conf.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{
		framesystem.InternalServiceName.String(),
		worldstatestore.Named("second").String(),
		worldstatestore.Named("first").String(),
	})

	// A name that is empty or repeated is rejected.
	_, _, err = (&Config{WorldStateStores: []string{"store", ""}}).Validate("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "world_state_stores[1] is empty")
	_, _, err = (&Config{WorldStateStores: []string{"store", "store"}}).Validate("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `names "store" more than once`)

	// The stores that are named are the ones used, in the config's order, whatever else is among
	// the dependencies.
	first, second := newNamedStore(t, "first"), newNamedStore(t, "second")
	other := inject.NewWorldStateStoreService("other")
	resourceConf := resource.Config{Name: "builtin", API: motion.API, ConvertedAttributes: conf}
	ms := &builtIn{logger: logging.NewTestLogger(t)}
	err = ms.BuiltInReconfigure(ctx, resource.Dependencies{
		worldstatestore.Named("first"):  first,
		worldstatestore.Named("other"):  other,
		worldstatestore.Named("second"): second,
	}, resourceConf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms.worldStateStores, test.ShouldHaveLength, 2)
	test.That(t, ms.worldStateStores[0], test.ShouldEqual, second)
	test.That(t, ms.worldStateStores[1], test.ShouldEqual, first)

	// A named store that is missing is an error.
	err = ms.BuiltInReconfigure(ctx, resource.Dependencies{worldstatestore.Named("second"): second}, resourceConf)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `world state store "first"`)

	// Reconfiguring without stores named stops using them.
	resourceConf.ConvertedAttributes = &Config{}
	test.That(t, ms.BuiltInReconfigure(ctx, nil, resourceConf), test.ShouldBeNil)
	test.That(t, ms.worldStateStores, test.ShouldBeEmpty)
}

// planRequestWithStore plans a move on the moving arm test robot, with its motion service reading
// a store that holds the given transforms.
func planRequestWithStore(t *testing.T, req motion.MoveReq, transforms ...*commonpb.Transform) error {
	t.Helper()

	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()
	ms.(*builtIn).worldStateStores = []worldstatestore.Service{newStore(t, transforms...)}
	_, err := ms.(*builtIn).plan(context.Background(), req, logging.NewTestLogger(t))
	return err
}

// TestMoveWithWorldStateStore plans a real move against a store, to assert that what the store
// holds decides whether the move can be planned.
func TestMoveWithWorldStateStore(t *testing.T) {
	moveReq := motion.MoveReq{
		ComponentName: "pieceGripper",
		Destination:   referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50})),
	}

	planWithStore := func(t *testing.T, transforms ...*commonpb.Transform) error {
		t.Helper()
		return planRequestWithStore(t, moveReq, transforms...)
	}

	t.Run("an empty store changes nothing", func(t *testing.T) {
		test.That(t, planWithStore(t), test.ShouldBeNil)
	})

	t.Run("an obstacle out of the way is planned around", func(t *testing.T) {
		test.That(t, planWithStore(t, storeTransform("far", referenceframe.World, r3.Vector{X: 5000, Y: 5000, Z: 5000}, 100)), test.ShouldBeNil)
	})

	// The same move and obstacle as TestArmMoveWithObstacles, with the obstacle coming from the
	// store instead of the request: a slab between the arm and its goal. The move can be planned
	// without the slab and cannot with it, which shows the planner received the store's obstacle.
	t.Run("an obstacle in the way blocks the move", func(t *testing.T) {
		blockedReq := motion.MoveReq{
			ComponentName: "pieceArm",
			Destination: referenceframe.NewPoseInFrame(
				referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}),
			),
		}
		test.That(t, planRequestWithStore(t, blockedReq), test.ShouldBeNil)

		slab := storeTransform("slab", referenceframe.World, r3.Vector{Z: 370}, 0)
		slab.PhysicalObject = &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: 2000, Y: 2000, Z: 20}},
			},
		}
		test.That(t, planRequestWithStore(t, blockedReq, slab), test.ShouldNotBeNil)
	})

	// The same slab marked visual only is not in the way.
	t.Run("a visual-only obstacle in the way does not block the move", func(t *testing.T) {
		blockedReq := motion.MoveReq{
			ComponentName: "pieceArm",
			Destination: referenceframe.NewPoseInFrame(
				referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}),
			),
		}
		slab := visualOnly(t, storeTransform("slab", referenceframe.World, r3.Vector{Z: 370}, 0))
		slab.PhysicalObject = &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: 2000, Y: 2000, Z: 20}},
			},
		}
		test.That(t, planRequestWithStore(t, blockedReq, slab), test.ShouldBeNil)
	})

	t.Run("store contents the planner cannot use do not fail the move", func(t *testing.T) {
		test.That(t, planWithStore(t,
			storeTransform("orphan", "nonexistent_frame", r3.Vector{}, 100),
			storeTransform("orphan-frame", "nonexistent_frame", r3.Vector{}, 0),
			storeTransform("pieceArm", referenceframe.World, r3.Vector{}, 0),
		), test.ShouldBeNil)
	})
}

// TestMoveWithConfiguredWorldStateStore builds a robot from a config that has a world state store
// and a motion service naming it, to assert that the config alone connects the two.
func TestMoveWithConfiguredWorldStateStore(t *testing.T) {
	ctx := context.Background()

	// Add the two services to the moving arm config.
	raw, err := os.ReadFile("../data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	var cfg map[string]interface{}
	test.That(t, json.Unmarshal(raw, &cfg), test.ShouldBeNil)
	cfg["services"] = []interface{}{
		map[string]interface{}{"name": "world-state", "api": worldstatestore.API.String(), "model": "rdk:builtin:builtin"},
		map[string]interface{}{
			"name":       "builtin",
			"api":        motion.API.String(),
			"model":      "rdk:builtin:builtin",
			"attributes": map[string]interface{}{"world_state_stores": []interface{}{"world-state"}},
		},
	}
	raw, err = json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	test.That(t, os.WriteFile(cfgPath, raw, 0o600), test.ShouldBeNil)

	ms, teardown := setupMotionServiceFromConfig(t, cfgPath)
	defer teardown()
	test.That(t, ms.(*builtIn).worldStateStores, test.ShouldHaveLength, 1)
	store := ms.(*builtIn).worldStateStores[0]
	test.That(t, store, test.ShouldNotBeNil)
	test.That(t, store.Name(), test.ShouldResemble, worldstatestore.Named("world-state"))

	// The move is possible while the store is empty.
	moveReq := motion.MoveReq{
		ComponentName: "pieceArm",
		Destination: referenceframe.NewPoseInFrame(
			referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}),
		),
	}
	_, err = ms.(*builtIn).plan(ctx, moveReq, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)

	// Adding a slab between the arm and its goal through the store's DoCommand makes the same move
	// impossible, with nothing passed in the request.
	_, err = store.DoCommand(ctx, map[string]interface{}{wssbuiltin.DoAdd: []interface{}{
		map[string]interface{}{
			"referenceFrame": "slab",
			"poseInObserverFrame": map[string]interface{}{
				"referenceFrame": referenceframe.World,
				"pose":           map[string]interface{}{"z": 370.0, "oZ": 1.0},
			},
			"physicalObject": map[string]interface{}{
				"box": map[string]interface{}{"dimsMm": map[string]interface{}{"x": 2000.0, "y": 2000.0, "z": 20.0}},
			},
		},
	}})
	test.That(t, err, test.ShouldBeNil)
	_, err = ms.(*builtIn).plan(ctx, moveReq, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldNotBeNil)
}

// TestMoveWithStoreConfiguredTransforms asserts that an obstacle listed in the store's config
// blocks a move from the moment the robot starts, with nothing added at runtime.
func TestMoveWithStoreConfiguredTransforms(t *testing.T) {
	raw, err := os.ReadFile("../data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	var cfg map[string]interface{}
	test.That(t, json.Unmarshal(raw, &cfg), test.ShouldBeNil)
	cfg["services"] = []interface{}{
		map[string]interface{}{
			"name":  "world-state",
			"api":   worldstatestore.API.String(),
			"model": "rdk:builtin:builtin",
			"attributes": map[string]interface{}{"transforms": []interface{}{
				map[string]interface{}{
					"referenceFrame": "slab",
					"poseInObserverFrame": map[string]interface{}{
						"referenceFrame": referenceframe.World,
						"pose":           map[string]interface{}{"z": 370.0, "oZ": 1.0},
					},
					"physicalObject": map[string]interface{}{
						"box": map[string]interface{}{"dimsMm": map[string]interface{}{"x": 2000.0, "y": 2000.0, "z": 20.0}},
					},
				},
			}},
		},
		map[string]interface{}{
			"name":       "builtin",
			"api":        motion.API.String(),
			"model":      "rdk:builtin:builtin",
			"attributes": map[string]interface{}{"world_state_stores": []interface{}{"world-state"}},
		},
	}
	raw, err = json.Marshal(cfg)
	test.That(t, err, test.ShouldBeNil)
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	test.That(t, os.WriteFile(cfgPath, raw, 0o600), test.ShouldBeNil)

	ms, teardown := setupMotionServiceFromConfig(t, cfgPath)
	defer teardown()

	// The store holds the configured slab, under its reference frame name.
	slab, err := ms.(*builtIn).worldStateStores[0].GetTransform(context.Background(), []byte("slab"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, slab.GetPhysicalObject().GetBox().GetDimsMm().GetX(), test.ShouldEqual, 2000)

	// The move that TestMoveWithConfiguredWorldStateStore can plan with an empty store is blocked.
	moveReq := motion.MoveReq{
		ComponentName: "pieceArm",
		Destination: referenceframe.NewPoseInFrame(
			referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}),
		),
	}
	_, err = ms.(*builtIn).plan(context.Background(), moveReq, logging.NewTestLogger(t))
	test.That(t, err, test.ShouldNotBeNil)
}
