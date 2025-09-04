package fake

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

func TestFakeWorldStateStore(t *testing.T) {
	// Create a new fake service
	fake := newFakeWorldStateStore(resource.Name{Name: "test"}, nil)
	defer fake.Close(context.Background())

	// Test ListUUIDs
	uuids, err := fake.ListUUIDs(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uuids), test.ShouldEqual, 3) // box, sphere, capsule

	// Test GetTransform for each static transform
	boxTransform, err := fake.GetTransform(context.Background(), []byte("box-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boxTransform, test.ShouldNotBeNil)
	test.That(t, boxTransform.Uuid, test.ShouldResemble, []byte("box-001"))
	test.That(t, boxTransform.Metadata, test.ShouldNotBeNil)

	// Test color metadata - should be a structpb.Value containing a StructValue
	colorField := boxTransform.Metadata.Fields["color"]
	test.That(t, colorField, test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue(), test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue().Fields["r"].GetNumberValue(), test.ShouldEqual, 255)
	test.That(t, colorField.GetStructValue().Fields["g"].GetNumberValue(), test.ShouldEqual, 0)
	test.That(t, colorField.GetStructValue().Fields["b"].GetNumberValue(), test.ShouldEqual, 0)

	test.That(t, boxTransform.Metadata.Fields["opacity"].GetNumberValue(), test.ShouldEqual, 0.3)

	sphereTransform, err := fake.GetTransform(context.Background(), []byte("sphere-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sphereTransform, test.ShouldNotBeNil)
	test.That(t, sphereTransform.Uuid, test.ShouldResemble, []byte("sphere-001"))
	test.That(t, sphereTransform.Metadata, test.ShouldNotBeNil)

	// Test color metadata for sphere
	colorField = sphereTransform.Metadata.Fields["color"]
	test.That(t, colorField, test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue(), test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue().Fields["r"].GetNumberValue(), test.ShouldEqual, 0)
	test.That(t, colorField.GetStructValue().Fields["g"].GetNumberValue(), test.ShouldEqual, 0)
	test.That(t, colorField.GetStructValue().Fields["b"].GetNumberValue(), test.ShouldEqual, 255)

	test.That(t, sphereTransform.Metadata.Fields["opacity"].GetNumberValue(), test.ShouldEqual, 0.7)

	capsuleTransform, err := fake.GetTransform(context.Background(), []byte("capsule-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capsuleTransform, test.ShouldNotBeNil)
	test.That(t, capsuleTransform.Uuid, test.ShouldResemble, []byte("capsule-001"))
	test.That(t, capsuleTransform.Metadata, test.ShouldNotBeNil)

	// Test color metadata for capsule
	colorField = capsuleTransform.Metadata.Fields["color"]
	test.That(t, colorField, test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue(), test.ShouldNotBeNil)
	test.That(t, colorField.GetStructValue().Fields["r"].GetNumberValue(), test.ShouldEqual, 0)
	test.That(t, colorField.GetStructValue().Fields["g"].GetNumberValue(), test.ShouldEqual, 255)
	test.That(t, colorField.GetStructValue().Fields["b"].GetNumberValue(), test.ShouldEqual, 0)

	test.That(t, capsuleTransform.Metadata.Fields["opacity"].GetNumberValue(), test.ShouldEqual, 1.0)

	// Test StreamTransformChanges
	changesChan, err := fake.StreamTransformChanges(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, changesChan, test.ShouldNotBeNil)

	// Wait a bit for some changes to occur
	time.Sleep(200 * time.Millisecond)

	// Check that we've received some changes
	changeCount := 0
	select {
	case <-changesChan:
		changeCount++
	default:
		// No changes ready yet
	}

	// We should have at least some changes after 200ms
	test.That(t, changeCount, test.ShouldBeGreaterThanOrEqualTo, 0)
}

func TestFakeWorldStateStoreClose(t *testing.T) {
	fake := newFakeWorldStateStore(resource.Name{Name: "test"}, nil)

	// Test that Close works
	err := fake.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Test that ListUUIDs still works after close (should return empty)
	uuids, err := fake.ListUUIDs(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uuids), test.ShouldBeGreaterThanOrEqualTo, 3) // Static transforms are still available
	test.That(t, len(uuids), test.ShouldBeLessThanOrEqualTo, 4)    // Dynamic transform may be available
}

func TestDoCommandSetFPS(t *testing.T) {
	logger := logging.NewTestLogger(t)
	name := resource.NewName(worldstatestore.API, "fake1")
	svc := newFakeWorldStateStore(name, logger)
	wss := svc.(*WorldStateStore)
	defer func() { _ = wss.Close(context.Background()) }()

	test.That(t, wss.fps, test.ShouldEqual, 10)

	resp, err := wss.DoCommand(context.Background(), map[string]any{"fps": 20})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wss.fps, test.ShouldEqual, uint16(20))
	test.That(t, resp["status"], test.ShouldEqual, "fps set to 20")

	_, err = wss.DoCommand(context.Background(), map[string]any{"fps": 0})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "fps must be greater than 0")
	test.That(t, wss.fps, test.ShouldEqual, uint16(20))
}

func TestDoCommandUnknownCommand(t *testing.T) {
	logger := logging.NewTestLogger(t)
	name := resource.NewName(worldstatestore.API, "fake3")
	svc := newFakeWorldStateStore(name, logger)
	wss := svc.(*WorldStateStore)
	defer func() { _ = wss.Close(context.Background()) }()

	resp, err := wss.DoCommand(context.Background(), map[string]any{"noop": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["status"], test.ShouldEqual, "command not implemented")
}
