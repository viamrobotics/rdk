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
	fake := newFakeWorldStateStore(resource.Name{Name: "test"}, nil, nil)
	defer fake.Close(context.Background())

	// Test ListUUIDs
	uuids, err := fake.ListUUIDs(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uuids), test.ShouldBeGreaterThanOrEqualTo, 3) // Static transforms are available
	test.That(t, len(uuids), test.ShouldBeLessThanOrEqualTo, 4)    // Dynamic transform may be available

	// Test GetTransform for each static transform
	boxTransform, err := fake.GetTransform(context.Background(), []byte("box-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, boxTransform, test.ShouldNotBeNil)
	test.That(t, boxTransform.Uuid, test.ShouldResemble, []byte("box-001"))

	sphereTransform, err := fake.GetTransform(context.Background(), []byte("sphere-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sphereTransform, test.ShouldNotBeNil)
	test.That(t, sphereTransform.Uuid, test.ShouldResemble, []byte("sphere-001"))
	test.That(t, sphereTransform.Metadata, test.ShouldNotBeNil)

	capsuleTransform, err := fake.GetTransform(context.Background(), []byte("capsule-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, capsuleTransform, test.ShouldNotBeNil)
	test.That(t, capsuleTransform.Uuid, test.ShouldResemble, []byte("capsule-001"))
	test.That(t, capsuleTransform.Metadata, test.ShouldNotBeNil)

	// Test StreamTransformChanges
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := fake.StreamTransformChanges(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, stream, test.ShouldNotBeNil)
}

func TestFakeWorldStateStoreClose(t *testing.T) {
	fake := newFakeWorldStateStore(resource.Name{Name: "test"}, nil, nil)

	// Test that Close works
	err := fake.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}

func TestDoCommandSetFPS(t *testing.T) {
	logger := logging.NewTestLogger(t)
	name := resource.NewName(worldstatestore.API, "fake1")
	svc := newFakeWorldStateStore(name, nil, logger)
	wss := svc.(*WorldStateStore)
	defer func() { _ = wss.Close(context.Background()) }()

	test.That(t, wss.fps, test.ShouldEqual, 10)

	// set fps via DoCommand
	resp, err := wss.DoCommand(context.Background(), map[string]any{"fps": float64(20)})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wss.fps, test.ShouldEqual, 20)
	test.That(t, resp["status"], test.ShouldEqual, "fps set to 20.00")

	// attempt to set invalid fps
	_, err = wss.DoCommand(context.Background(), map[string]any{"fps": float64(0)})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "fps must be greater than 0")
	test.That(t, wss.fps, test.ShouldEqual, 20)
}

func TestDoCommandUnknownCommand(t *testing.T) {
	logger := logging.NewTestLogger(t)
	name := resource.NewName(worldstatestore.API, "fake3")
	svc := newFakeWorldStateStore(name, nil, logger)
	wss := svc.(*WorldStateStore)
	defer func() { _ = wss.Close(context.Background()) }()

	resp, err := wss.DoCommand(context.Background(), map[string]any{"noop": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["status"], test.ShouldEqual, "command not implemented")
}
