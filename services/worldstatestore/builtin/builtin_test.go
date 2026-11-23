package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"slices"
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/structpb"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// event pairs a transform with the updated fields to send alongside it in a DoCommand list item.
type event = pb.StreamTransformChangesResponse

// command builds the DoCommand payload for `name` the way a client would: each transform in its
// proto JSON form, with any updated fields beside it as a list of paths.
func command(t *testing.T, name string, events ...*event) map[string]interface{} {
	t.Helper()

	items := make([]interface{}, 0, len(events))
	for _, ev := range events {
		encoded, err := protojson.Marshal(ev.GetTransform())
		test.That(t, err, test.ShouldBeNil)
		var item map[string]interface{}
		test.That(t, json.Unmarshal(encoded, &item), test.ShouldBeNil)
		if ev.GetUpdatedFields() != nil {
			paths := make([]interface{}, 0, len(ev.GetUpdatedFields().GetPaths()))
			for _, path := range ev.GetUpdatedFields().GetPaths() {
				paths = append(paths, path)
			}
			item[updatedFieldsKey] = paths
		}
		items = append(items, item)
	}
	return map[string]interface{}{name: items}
}

func boxTransform(uuid string, x float64) *commonpb.Transform {
	return &commonpb.Transform{
		Uuid:           []byte(uuid),
		ReferenceFrame: uuid + "-frame",
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: "world",
			Pose:           &commonpb.Pose{X: x, Y: 2, Z: 3, OZ: 1},
		},
		PhysicalObject: &commonpb.Geometry{
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{X: 10, Y: 20, Z: 30}},
			},
		},
	}
}

// nextChange returns the stream's next change, failing the test if none arrives promptly.
func nextChange(t *testing.T, stream *worldstatestore.TransformChangeStream) worldstatestore.TransformChange {
	t.Helper()

	type result struct {
		change worldstatestore.TransformChange
		err    error
	}
	future := make(chan result, 1)
	go func() {
		change, err := stream.Next()
		future <- result{change, err}
	}()

	select {
	case res := <-future:
		test.That(t, res.err, test.ShouldBeNil)
		return res.change
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a transform change")
		return worldstatestore.TransformChange{}
	}
}

func newTestStore(t *testing.T) *builtIn {
	t.Helper()

	svc := newBuiltIn(worldstatestore.Named("store"), logging.NewTestLogger(t))
	t.Cleanup(func() { test.That(t, svc.Close(context.Background()), test.ShouldBeNil) })
	return svc
}

// TestAddGetRemove walks a pair of transforms through the store: a bulk add, reads that cannot
// alias the store's memory, and a remove.
func TestAddGetRemove(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)

	// The store starts empty, with a non-nil list.
	uuids, err := svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldNotBeNil)
	test.That(t, uuids, test.ShouldBeEmpty)

	// Add two transforms in one call. The second has no uuid, so the store generates one.
	generated := boxTransform("generated", 0)
	generated.Uuid = nil
	resp, err := svc.DoCommand(ctx, command(t, DoAdd,
		&event{Transform: boxTransform("box", 1)},
		&event{Transform: generated},
	))
	test.That(t, err, test.ShouldBeNil)
	respUUIDs := resp[DoAdd].([]interface{})
	test.That(t, respUUIDs, test.ShouldHaveLength, 2)
	test.That(t, respUUIDs[0], test.ShouldEqual, "Ym94")
	test.That(t, respUUIDs[1], test.ShouldNotBeEmpty)

	uuids, err = svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldHaveLength, 2)

	// A returned transform is a copy: changing it does not change the store.
	got, err := svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, boxTransform("box", 1))
	got.PoseInObserverFrame.Pose.X = 999
	got, err = svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.GetPoseInObserverFrame().GetPose().GetX(), test.ShouldEqual, 1)

	// Remove it. Reads of it now fail, and the other transform is untouched.
	_, err = svc.DoCommand(ctx, command(t, DoRemove, &event{Transform: &commonpb.Transform{Uuid: []byte("box")}}))
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	uuids, err = svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldHaveLength, 1)
}

// TestUpdate asserts that an update only changes the fields its mask names, whichever spelling
// the paths use.
func TestUpdate(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)

	_, err := svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("box", 1)}))
	test.That(t, err, test.ShouldBeNil)

	// The patch also carries a `y`, but only `x` is named, so `y` must keep its stored value.
	_, err = svc.DoCommand(ctx, command(t, DoUpdate, &event{
		Transform: &commonpb.Transform{
			Uuid:                []byte("box"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 50, Y: 60}},
		},
		UpdatedFields: &fieldmaskpb.FieldMask{Paths: []string{"pose_in_observer_frame.pose.x"}},
	}))
	test.That(t, err, test.ShouldBeNil)

	expected := boxTransform("box", 50)
	got, err := svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, expected)

	// Without updated fields, an update changes exactly the fields it spells out. Here that is
	// one nested field, which switches the geometry's type, and the metadata, replaced whole.
	_, err = svc.DoCommand(ctx, map[string]interface{}{DoUpdate: []interface{}{
		map[string]interface{}{
			"uuid":           "Ym94",
			"physicalObject": map[string]interface{}{"sphere": map[string]interface{}{"radiusMm": 7.0}},
			"metadata":       map[string]interface{}{"color": "red"},
		},
	}})
	test.That(t, err, test.ShouldBeNil)

	expected.PhysicalObject = &commonpb.Geometry{GeometryType: &commonpb.Geometry_Sphere{Sphere: &commonpb.Sphere{RadiusMm: 7}}}
	expected.Metadata, err = structpb.NewStruct(map[string]interface{}{"color": "red"})
	test.That(t, err, test.ShouldBeNil)
	got, err = svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, expected)

	// A field sent as null is cleared.
	_, err = svc.DoCommand(ctx, map[string]interface{}{DoUpdate: []interface{}{
		map[string]interface{}{"uuid": "Ym94", "metadata": nil},
	}})
	test.That(t, err, test.ShouldBeNil)
	expected.Metadata = nil
	got, err = svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, expected)
}

// TestUpdateSharesWhatItDoesNotTouch asserts the two things that make an update cheap and safe.
// It shares the parts of a transform it does not reach, rather than copying them. And it never
// changes a transform or an event that already exists, which is what allows the sharing.
func TestUpdateSharesWhatItDoesNotTouch(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)
	stored := func() *commonpb.Transform {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.transforms["cloud"]
	}
	withPoints := func(points []byte) *commonpb.Geometry {
		return &commonpb.Geometry{GeometryType: &commonpb.Geometry_Pointcloud{
			Pointcloud: &commonpb.PointCloud{PointCloud: points},
		}}
	}
	update := func(patch *commonpb.Transform) {
		t.Helper()
		patch.Uuid = []byte("cloud")
		_, err := svc.DoCommand(ctx, command(t, DoUpdate, &event{Transform: patch}))
		test.That(t, err, test.ShouldBeNil)
	}

	cloud := boxTransform("cloud", 1)
	cloud.PhysicalObject = withPoints([]byte{1, 2, 3, 4})
	metadata, err := structpb.NewStruct(map[string]interface{}{"color": "red"})
	test.That(t, err, test.ShouldBeNil)
	cloud.Metadata = metadata
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: cloud}))
	test.That(t, err, test.ShouldBeNil)
	stream, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	nextChange(t, stream)

	// Move the transform. The store holds a new transform, and the old one is exactly as it was.
	original := stored()
	originalCopy := proto.Clone(original)
	update(&commonpb.Transform{PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 50}}})
	moved := stored()
	test.That(t, moved != original, test.ShouldBeTrue)
	test.That(t, original, test.ShouldResembleProto, originalCopy)
	test.That(t, moved.GetPoseInObserverFrame().GetPose().GetX(), test.ShouldEqual, 50)
	test.That(t, moved.GetPoseInObserverFrame().GetPose().GetY(), test.ShouldEqual, 2)

	// Only the pose was reached, so only the pose is new. The point cloud and the metadata are
	// the very same memory as before: moving a transform does not copy its geometry.
	test.That(t, moved.GetPoseInObserverFrame() != original.GetPoseInObserverFrame(), test.ShouldBeTrue)
	test.That(t, moved.GetPhysicalObject() == original.GetPhysicalObject(), test.ShouldBeTrue)
	test.That(t, moved.GetMetadata() == original.GetMetadata(), test.ShouldBeTrue)

	// Move it again, twice in one command. Neither the transform stored after the first move nor
	// the event published for it changes, though both shared memory with the store.
	movedEvent := nextChange(t, stream).Transform
	movedCopy, movedEventCopy := proto.Clone(moved), proto.Clone(movedEvent)
	_, err = svc.DoCommand(ctx, command(t, DoUpdate,
		&event{Transform: &commonpb.Transform{
			Uuid:                []byte("cloud"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 60}},
		}},
		&event{Transform: &commonpb.Transform{
			Uuid:                []byte("cloud"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{Y: 70}},
		}},
	))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, moved, test.ShouldResembleProto, movedCopy)
	test.That(t, movedEvent, test.ShouldResembleProto, movedEventCopy)
	movedTwice := stored()
	test.That(t, movedTwice.GetPoseInObserverFrame().GetPose().GetX(), test.ShouldEqual, 60)
	test.That(t, movedTwice.GetPoseInObserverFrame().GetPose().GetY(), test.ShouldEqual, 70)
	test.That(t, movedTwice.GetPhysicalObject() == original.GetPhysicalObject(), test.ShouldBeTrue)

	// Replace the points, which reaches inside the geometry. Now the geometry is new, the
	// transforms stored earlier keep the old points, and the metadata is still shared.
	update(&commonpb.Transform{PhysicalObject: withPoints([]byte{9, 9})})
	repointed := stored()
	test.That(t, repointed.GetPhysicalObject() != original.GetPhysicalObject(), test.ShouldBeTrue)
	test.That(t, repointed.GetPhysicalObject().GetPointcloud().GetPointCloud(), test.ShouldResemble, []byte{9, 9})
	test.That(t, original, test.ShouldResembleProto, originalCopy)
	test.That(t, movedTwice.GetPhysicalObject().GetPointcloud().GetPointCloud(), test.ShouldResemble, []byte{1, 2, 3, 4})
	test.That(t, repointed.GetMetadata() == original.GetMetadata(), test.ShouldBeTrue)
	test.That(t, repointed.GetPoseInObserverFrame().GetPose().GetX(), test.ShouldEqual, 60)

	// A caller's copy is still its own: changing it reaches nothing in the store.
	got, err := svc.GetTransform(ctx, []byte("cloud"), nil)
	test.That(t, err, test.ShouldBeNil)
	got.GetPhysicalObject().GetPointcloud().PointCloud[0] = 0
	test.That(t, stored().GetPhysicalObject().GetPointcloud().GetPointCloud(), test.ShouldResemble, []byte{9, 9})
}

// TestInvalidCommands asserts that every kind of invalid event is rejected, and that a rejected
// call leaves the store exactly as it was, including the valid events sent alongside.
func TestInvalidCommands(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)

	_, err := svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("box", 1)}))
	test.That(t, err, test.ShouldBeNil)

	stream, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextChange(t, stream).Transform.GetUuid(), test.ShouldResemble, []byte("box"))

	noFrame := boxTransform("no-frame", 0)
	noFrame.ReferenceFrame = ""
	noObserver := boxTransform("no-observer", 0)
	noObserver.PoseInObserverFrame.ReferenceFrame = ""
	update := func(uuid string, paths ...string) *event {
		return &event{
			Transform: &commonpb.Transform{
				Uuid:                []byte(uuid),
				PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 77}},
			},
			UpdatedFields: &fieldmaskpb.FieldMask{Paths: paths},
		}
	}
	remove := func(uuid string) *event {
		return &event{Transform: &commonpb.Transform{Uuid: []byte(uuid)}}
	}
	valid := &event{Transform: boxTransform("valid", 0)}
	validUpdate := update("box", "pose_in_observer_frame.pose.x")

	for _, tc := range []struct {
		name     string
		cmd      map[string]interface{}
		contains string
	}{
		{"not a list", map[string]interface{}{DoAdd: "box"}, "expected a list"},
		{"unknown json field", map[string]interface{}{DoAdd: []interface{}{map[string]interface{}{"nope": 1}}}, "add[0]"},
		{"not an object", map[string]interface{}{DoAdd: []interface{}{"box"}}, "expected a transform object"},
		{
			"updated fields not a list",
			map[string]interface{}{DoUpdate: []interface{}{map[string]interface{}{
				"uuid":          "Ym94",
				"updatedFields": "metadata",
			}}},
			"must be a list of field paths",
		},
		{"add without frame", command(t, DoAdd, valid, &event{Transform: noFrame}), "add[1]: transform has no reference frame"},
		{"add without observer", command(t, DoAdd, valid, &event{Transform: noObserver}), "no observer reference frame"},
		{"add existing", command(t, DoAdd, valid, &event{Transform: boxTransform("box", 0)}), "already exists"},
		{"add duplicate in batch", command(t, DoAdd, valid, valid), "add[1]"},
		{"update unknown uuid", command(t, DoUpdate, validUpdate, update("missing", "metadata")), "update[1]"},
		{"update nothing", command(t, DoUpdate, validUpdate, remove("box")), "at least one field"},
		{"update unknown path", command(t, DoUpdate, validUpdate, update("box", "pose_in_observer_frame.nope")), "no field"},
		{"update uuid", command(t, DoUpdate, validUpdate, update("box", "uuid")), "cannot be updated"},
		{"update inside scalar", command(t, DoUpdate, validUpdate, update("box", "reference_frame.x")), "cannot address"},
		{"remove unknown uuid", command(t, DoRemove, remove("box"), remove("missing")), "remove[1]"},
		{"remove twice in batch", command(t, DoRemove, remove("box"), remove("box")), "remove[1]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.DoCommand(ctx, tc.cmd)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.contains)
		})
	}

	// A command the store does not know is reported the standard way.
	_, err = svc.DoCommand(ctx, map[string]interface{}{"something_else": true})
	test.That(t, err, test.ShouldBeError, resource.ErrDoUnimplemented)

	// None of the rejected calls changed the store.
	uuids, err := svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldResemble, [][]byte{[]byte("box")})
	got, err := svc.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, boxTransform("box", 1))

	// Nor did they publish anything: the next change on the stream is the one made now.
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("marker", 0)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextChange(t, stream).Transform.GetUuid(), test.ShouldResemble, []byte("marker"))
}

// combine merges the payloads of several commands into the payload of one call.
func combine(commands ...map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for _, cmd := range commands {
		maps.Copy(ret, cmd)
	}
	return ret
}

// TestCombinedCommands asserts that one call can carry several commands, and that they are
// applied in the order remove, add, update, as a unit.
func TestCombinedCommands(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)
	encode := func(uuid string) string { return base64.StdEncoding.EncodeToString([]byte(uuid)) }
	remove := func(uuid string) *event {
		return &event{Transform: &commonpb.Transform{Uuid: []byte(uuid)}}
	}
	moveX := func(uuid string, x float64) *event {
		return &event{Transform: &commonpb.Transform{
			Uuid:                []byte(uuid),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: x}},
		}}
	}

	_, err := svc.DoCommand(ctx, command(t, DoAdd,
		&event{Transform: boxTransform("old", 1)},
		&event{Transform: boxTransform("keep", 1)},
	))
	test.That(t, err, test.ShouldBeNil)

	// Open a stream and read past its snapshot of the two transforms.
	stream, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	nextChange(t, stream)
	nextChange(t, stream)

	// One call replaces "old" under the same uuid, adds "new", and moves "new". Adding "old" only
	// works because the removal is applied first, and moving "new" because the add is.
	resp, err := svc.DoCommand(ctx, combine(
		command(t, DoUpdate, moveX("new", 9)),
		command(t, DoAdd, &event{Transform: boxTransform("old", 5)}, &event{Transform: boxTransform("new", 1)}),
		command(t, DoRemove, remove("old")),
	))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{
		DoRemove: []interface{}{encode("old")},
		DoAdd:    []interface{}{encode("old"), encode("new")},
		DoUpdate: []interface{}{encode("new")},
	})

	for uuid, expected := range map[string]*commonpb.Transform{
		"old":  boxTransform("old", 5),
		"new":  boxTransform("new", 9),
		"keep": boxTransform("keep", 1),
	} {
		got, err := svc.GetTransform(ctx, []byte(uuid), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldResembleProto, expected)
	}

	// The stream reports the changes in the order they were applied.
	for _, expected := range []struct {
		changeType pb.TransformChangeType
		uuid       string
	}{
		{pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED, "old"},
		{pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, "old"},
		{pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED, "new"},
		{pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED, "new"},
	} {
		change := nextChange(t, stream)
		test.That(t, change.ChangeType, test.ShouldEqual, expected.changeType)
		test.That(t, change.Transform.GetUuid(), test.ShouldResemble, []byte(expected.uuid))
	}

	// A call is applied as a unit: an invalid item in one command undoes the other commands too.
	_, err = svc.DoCommand(ctx, combine(
		command(t, DoRemove, remove("keep")),
		command(t, DoAdd, &event{Transform: boxTransform("extra", 1)}),
		command(t, DoUpdate, moveX("missing", 1)),
	))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `update[0]: transform "missing" not found`)

	// The order is fixed, so a transform cannot be updated in the call that removes it.
	_, err = svc.DoCommand(ctx, combine(command(t, DoRemove, remove("keep")), command(t, DoUpdate, moveX("keep", 3))))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `update[0]: transform "keep" not found`)

	// Neither rejected call changed the store or published anything.
	uuids, err := svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldHaveLength, 3)
	_, err = svc.GetTransform(ctx, []byte("keep"), nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("marker", 0)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextChange(t, stream).Transform.GetUuid(), test.ShouldResemble, []byte("marker"))

	// A command with an empty list is still a command.
	resp, err = svc.DoCommand(ctx, map[string]interface{}{DoAdd: []interface{}{}})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{DoAdd: []interface{}{}})
}

// TestStream asserts that every subscriber sees every change in order, and that a subscriber
// that joins late is first brought up to date.
func TestStream(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)

	first, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	second, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// Add, update and remove a transform.
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("box", 1)}))
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoUpdate, &event{
		Transform: &commonpb.Transform{
			Uuid:                []byte("box"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 50}},
		},
	}))
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoRemove, &event{Transform: &commonpb.Transform{Uuid: []byte("box")}}))
	test.That(t, err, test.ShouldBeNil)

	for _, stream := range []*worldstatestore.TransformChangeStream{first, second} {
		added := nextChange(t, stream)
		test.That(t, added.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
		test.That(t, added.Transform, test.ShouldResembleProto, boxTransform("box", 1))
		test.That(t, added.UpdatedFields, test.ShouldBeEmpty)

		// The update names the leaf that changed, in its proto spelling, and carries the whole
		// merged pose rather than only that leaf.
		updated := nextChange(t, stream)
		test.That(t, updated.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED)
		test.That(t, updated.UpdatedFields, test.ShouldResemble, []string{"pose_in_observer_frame.pose.x"})
		test.That(t, updated.Transform, test.ShouldResembleProto, &commonpb.Transform{
			Uuid:                []byte("box"),
			PoseInObserverFrame: boxTransform("box", 50).GetPoseInObserverFrame(),
		})

		removed := nextChange(t, stream)
		test.That(t, removed.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED)
		test.That(t, removed.Transform, test.ShouldResembleProto, &commonpb.Transform{Uuid: []byte("box")})
		test.That(t, removed.UpdatedFields, test.ShouldResemble, []string{"uuid"})
	}

	// A subscriber that joins after a transform was added receives it as an added change.
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("late", 4)}))
	test.That(t, err, test.ShouldBeNil)
	late, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	snapshot := nextChange(t, late)
	test.That(t, snapshot.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
	test.That(t, snapshot.Transform, test.ShouldResembleProto, boxTransform("late", 4))
}

// numSubscribers returns how many streams the store is feeding.
func numSubscribers(svc *builtIn) int {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	return len(svc.subscribers)
}

// TestStreamLifetime covers the three ways a stream ends: its context is canceled, its consumer
// falls too far behind, or the service closes.
func TestStreamLifetime(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)
	svc.backlogSlack = 1
	moveX := func(x float64) map[string]interface{} {
		return command(t, DoUpdate, &event{Transform: &commonpb.Transform{
			Uuid:                []byte("box"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: x}},
		}})
	}

	// Canceling a stream's context unsubscribes it.
	cancelCtx, cancel := context.WithCancel(ctx)
	canceled, err := svc.StreamTransformChanges(cancelCtx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 1)
	cancel()
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		test.That(tb, numSubscribers(svc), test.ShouldEqual, 0)
	})
	_, err = canceled.Next()
	test.That(t, err, test.ShouldBeError, context.Canceled)

	// A stream nobody reads is kept for as long as it would be cheaper to catch up than to start
	// over. With one transform stored and a slack of one, that is up to two unread changes.
	slow, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("box", 0)}))
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, moveX(1))
	test.That(t, err, test.ShouldBeNil)
	// The stream now holds two unread changes, which is still within bounds, so a third is queued.
	_, err = svc.DoCommand(ctx, moveX(2))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 1)
	// With three unread it is past the bound when the next command arrives, and is dropped. Its
	// stream ends at once, without first handing over the changes it never read.
	_, err = svc.DoCommand(ctx, moveX(3))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 0)
	_, err = slow.Next()
	test.That(t, err, test.ShouldEqual, io.EOF)

	// A stream opened afterwards starts from the store as it is now.
	fresh, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextChange(t, fresh).Transform, test.ShouldResembleProto, boxTransform("box", 3))

	// A stream waiting for a change is woken by the service closing. Closing also refuses new
	// streams and commands, and closing again is harmless.
	waiting := make(chan error, 1)
	go func() {
		_, err := fresh.Next()
		waiting <- err
	}()
	select {
	case err := <-waiting:
		t.Fatalf("the stream returned before anything happened: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	select {
	case err := <-waiting:
		test.That(t, err, test.ShouldEqual, io.EOF)
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not end when the service closed")
	}
	_, err = svc.StreamTransformChanges(ctx, nil)
	test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("three", 0)}))
	test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
}

// TestStreamLargeCommands asserts that the size of a command or of the store never ends a
// stream. Only the changes a stream left unread before a command count against it.
func TestStreamLargeCommands(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)
	svc.backlogSlack = 1
	const numTransforms = 50

	adds := make([]*event, 0, numTransforms)
	removes := make([]*event, 0, numTransforms)
	for idx := range numTransforms {
		uuid := fmt.Sprintf("box-%d", idx)
		adds = append(adds, &event{Transform: boxTransform(uuid, float64(idx))})
		removes = append(removes, &event{Transform: &commonpb.Transform{Uuid: []byte(uuid)}})
	}
	readAll := func(stream *worldstatestore.TransformChangeStream, changeType pb.TransformChangeType) []string {
		t.Helper()
		uuids := make([]string, 0, numTransforms)
		for range numTransforms {
			change := nextChange(t, stream)
			test.That(t, change.ChangeType, test.ShouldEqual, changeType)
			uuids = append(uuids, string(change.Transform.GetUuid()))
		}
		return uuids
	}
	inOrder := make([]string, 0, numTransforms)
	for _, add := range adds {
		inOrder = append(inOrder, string(add.GetTransform().GetUuid()))
	}

	// One add far larger than the slack reaches a stream that reads none of it until afterwards.
	early, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, adds...))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 1)
	test.That(t, readAll(early, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED), test.ShouldResemble, inOrder)

	// A stream opened on a store far larger than the slack is sent all of it, and survives the
	// next command even though it has not read any of it yet.
	late, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("one-more", 0)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 2)
	snapshot := readAll(late, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
	slices.Sort(snapshot)
	expected := slices.Clone(inOrder)
	slices.Sort(expected)
	test.That(t, snapshot, test.ShouldResemble, expected)
	test.That(t, nextChange(t, late).Transform.GetUuid(), test.ShouldResemble, []byte("one-more"))
	test.That(t, nextChange(t, early).Transform.GetUuid(), test.ShouldResemble, []byte("one-more"))

	// One remove that empties most of the store reaches both streams, in order.
	_, err = svc.DoCommand(ctx, command(t, DoRemove, removes...))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(svc), test.ShouldEqual, 2)
	test.That(t, readAll(early, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED), test.ShouldResemble, inOrder)
	test.That(t, readAll(late, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED), test.ShouldResemble, inOrder)
}

// TestOverGRPC drives the store through the gRPC server and client, where a DoCommand arrives
// as a protobuf struct rather than as Go values.
func TestOverGRPC(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	name := worldstatestore.Named("store")

	// Serve the model as it is registered.
	registration, ok := resource.LookupRegistration(worldstatestore.API, resource.DefaultServiceModel)
	test.That(t, ok, test.ShouldBeTrue)
	res, err := registration.Constructor(
		ctx, nil, resource.Config{Name: name.Name, API: worldstatestore.API, ConvertedAttributes: &Config{}}, logger,
	)
	test.That(t, err, test.ShouldBeNil)
	svc := res.(worldstatestore.Service)
	defer func() { test.That(t, svc.Close(ctx), test.ShouldBeNil) }()

	//nolint: noctx
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)
	coll, err := resource.NewAPIResourceCollection(worldstatestore.API, map[resource.Name]worldstatestore.Service{name: svc})
	test.That(t, err, test.ShouldBeNil)
	api, ok, err := resource.LookupAPIRegistration[worldstatestore.Service](worldstatestore.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, api.RegisterRPCService(ctx, rpcServer, coll, logger), test.ShouldBeNil)
	go rpcServer.Serve(listener)
	defer rpcServer.Stop()

	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()
	client, err := worldstatestore.NewClientFromConn(ctx, conn, "", name, logger)
	test.That(t, err, test.ShouldBeNil)

	// Listing an empty store succeeds.
	uuids, err := client.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldBeEmpty)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := client.StreamTransformChanges(streamCtx, nil)
	test.That(t, err, test.ShouldBeNil)

	// Add and update through the client, and read the result back.
	resp, err := client.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("box", 1)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp, test.ShouldResemble, map[string]interface{}{DoAdd: []interface{}{"Ym94"}})
	_, err = client.DoCommand(ctx, command(t, DoUpdate, &event{
		Transform: &commonpb.Transform{
			Uuid:                []byte("box"),
			PoseInObserverFrame: &commonpb.PoseInFrame{Pose: &commonpb.Pose{X: 50}},
		},
		UpdatedFields: &fieldmaskpb.FieldMask{Paths: []string{"pose_in_observer_frame.pose.x"}},
	}))
	test.That(t, err, test.ShouldBeNil)

	uuids, err = client.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldResemble, [][]byte{[]byte("box")})
	got, err := client.GetTransform(ctx, []byte("box"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldResembleProto, boxTransform("box", 50))

	// The stream delivered both changes.
	added := nextChange(t, stream)
	test.That(t, added.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
	test.That(t, added.Transform, test.ShouldResembleProto, boxTransform("box", 1))
	updated := nextChange(t, stream)
	test.That(t, updated.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED)
	test.That(t, updated.UpdatedFields, test.ShouldResemble, []string{"pose_in_observer_frame.pose.x"})
}

// TestConfiguredTransforms asserts that a store starts with the transforms in its config, and
// that a config it could not start with is rejected.
func TestConfiguredTransforms(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	table := map[string]any{
		"referenceFrame":      "table",
		"poseInObserverFrame": map[string]any{"referenceFrame": "world", "pose": map[string]any{"z": -10.0}},
		"physicalObject":      map[string]any{"box": map[string]any{"dimsMm": map[string]any{"x": 1000.0, "y": 1000.0, "z": 20.0}}},
	}
	wall := map[string]any{
		"uuid":                "d2FsbA==",
		"referenceFrame":      "wall",
		"poseInObserverFrame": map[string]any{"referenceFrame": "world"},
	}
	conf := &Config{Transforms: []map[string]any{table, wall}}
	_, _, err := conf.Validate("")
	test.That(t, err, test.ShouldBeNil)

	// Build the store the way the robot does.
	registration, ok := resource.LookupRegistration(worldstatestore.API, resource.DefaultServiceModel)
	test.That(t, ok, test.ShouldBeTrue)
	res, err := registration.Constructor(
		ctx, nil, resource.Config{Name: "store", API: worldstatestore.API, ConvertedAttributes: conf}, logger,
	)
	test.That(t, err, test.ShouldBeNil)
	svc := res.(worldstatestore.Service)
	defer func() { test.That(t, svc.Close(ctx), test.ShouldBeNil) }()

	// Both transforms are there. The one without a uuid is stored under its reference frame name.
	uuids, err := svc.ListUUIDs(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, uuids, test.ShouldHaveLength, 2)
	got, err := svc.GetTransform(ctx, []byte("table"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got.GetPoseInObserverFrame().GetPose().GetZ(), test.ShouldEqual, -10)
	test.That(t, got.GetPhysicalObject().GetBox().GetDimsMm().GetX(), test.ShouldEqual, 1000)
	_, err = svc.GetTransform(ctx, []byte("wall"), nil)
	test.That(t, err, test.ShouldBeNil)

	// A new stream starts with them, and they can be changed like any other transform.
	stream, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, nextChange(t, stream).ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
	test.That(t, nextChange(t, stream).ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
	_, err = svc.DoCommand(ctx, command(t, DoRemove, &event{Transform: &commonpb.Transform{Uuid: []byte("table")}}))
	test.That(t, err, test.ShouldBeNil)

	// Validation names the transform that is wrong: here the second repeats the first's identity.
	_, _, err = (&Config{Transforms: []map[string]any{table, table}}).Validate("services.0")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `transforms[1]: transform "table" already exists`)
	_, _, err = (&Config{Transforms: []map[string]any{{"referenceFrame": "floating"}}}).Validate("services.0")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "transforms[0]: transform has no observer reference frame")

	// An empty config is an empty store.
	_, _, err = (&Config{}).Validate("")
	test.That(t, err, test.ShouldBeNil)
}
