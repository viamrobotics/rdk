package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/encoding/protojson"
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
	respUUIDs := resp[UUIDsKey].([]interface{})
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
		{"two commands", map[string]interface{}{DoAdd: []interface{}{}, DoRemove: []interface{}{}}, "only one"},
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

// TestStreamLifetime covers the three ways a stream ends: its context is canceled, its consumer
// falls too far behind, or the service closes.
func TestStreamLifetime(t *testing.T) {
	ctx := context.Background()
	svc := newTestStore(t)
	svc.subscriberBuffer = 1

	numSubscribers := func() int {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return len(svc.subscribers)
	}
	waitForSubscribers := func(expected int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for numSubscribers() != expected {
			if time.Now().After(deadline) {
				t.Fatalf("expected %d subscribers, have %d", expected, numSubscribers())
			}
			time.Sleep(time.Millisecond)
		}
	}

	// Canceling a stream's context unsubscribes it.
	cancelCtx, cancel := context.WithCancel(ctx)
	canceled, err := svc.StreamTransformChanges(cancelCtx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(), test.ShouldEqual, 1)
	cancel()
	waitForSubscribers(0)
	_, err = canceled.Next()
	test.That(t, err, test.ShouldNotBeNil)

	// A stream nobody reads fills its one-change buffer on the first add. The second add cannot
	// be delivered, so the stream is disconnected: it yields what it had, then ends.
	slow, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("one", 0)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(), test.ShouldEqual, 1)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("two", 0)}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, numSubscribers(), test.ShouldEqual, 0)
	test.That(t, nextChange(t, slow).Transform.GetUuid(), test.ShouldResemble, []byte("one"))
	_, err = slow.Next()
	test.That(t, err, test.ShouldEqual, io.EOF)

	// Closing the service ends open streams once they drain, and refuses new streams and
	// commands. Closing again is harmless.
	open, err := svc.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	test.That(t, svc.Close(ctx), test.ShouldBeNil)
	for {
		_, err = open.Next()
		if err != nil {
			break
		}
	}
	test.That(t, err, test.ShouldEqual, io.EOF)
	_, err = svc.StreamTransformChanges(ctx, nil)
	test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
	_, err = svc.DoCommand(ctx, command(t, DoAdd, &event{Transform: boxTransform("three", 0)}))
	test.That(t, errors.Is(err, errClosed), test.ShouldBeTrue)
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
	test.That(t, resp[UUIDsKey], test.ShouldResemble, []interface{}{"Ym94"})
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
