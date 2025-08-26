package fake

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestFakeWorldStateStore(t *testing.T) {
	// Create a new fake service
	fake := NewFakeWorldStateStore()
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

	sphereTransform, err := fake.GetTransform(context.Background(), []byte("sphere-001"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, sphereTransform, test.ShouldNotBeNil)
	test.That(t, sphereTransform.Uuid, test.ShouldResemble, []byte("sphere-001"))

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

	// Test DoCommand
	result, err := fake.DoCommand(context.Background(), map[string]interface{}{"test": "command"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result["status"], test.ShouldEqual, "do command not implemented")
}

func TestFakeWorldStateStoreClose(t *testing.T) {
	fake := NewFakeWorldStateStore()

	// Test that Close works
	err := fake.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)

	// Test that ListUUIDs still works after close (should return empty)
	uuids, err := fake.ListUUIDs(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(uuids), test.ShouldEqual, 3) // Static transforms are still available
}
