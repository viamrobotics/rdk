package worldstatestore_test

import (
	"context"
	"errors"
	"io"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/services/worldstatestore"
)

func TestTransformChangeStream(t *testing.T) {
	t.Run("a nil stream has ended", func(t *testing.T) {
		var stream *worldstatestore.TransformChangeStream
		_, err := stream.Next()
		test.That(t, err, test.ShouldEqual, io.EOF)
	})

	t.Run("a stream built from a function returns what the function returns", func(t *testing.T) {
		boom := errors.New("boom")
		results := []struct {
			change worldstatestore.TransformChange
			err    error
		}{
			{change: worldstatestore.TransformChange{
				ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
				Transform:  &commonpb.Transform{Uuid: []byte("a")},
			}},
			{err: boom},
			{err: io.EOF},
		}
		calls := 0
		stream := worldstatestore.NewTransformChangeStream(func() (worldstatestore.TransformChange, error) {
			result := results[calls]
			calls++
			return result.change, result.err
		})

		change, err := stream.Next()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, change.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED)
		test.That(t, change.Transform.GetUuid(), test.ShouldResemble, []byte("a"))
		_, err = stream.Next()
		test.That(t, err, test.ShouldEqual, boom)
		_, err = stream.Next()
		test.That(t, err, test.ShouldEqual, io.EOF)
	})

	t.Run("a stream built from a channel ends when the channel closes or the context is done", func(t *testing.T) {
		changes := make(chan worldstatestore.TransformChange, 1)
		changes <- worldstatestore.TransformChange{ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED}
		close(changes)
		stream := worldstatestore.NewTransformChangeStreamFromChannel(context.Background(), changes)
		change, err := stream.Next()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, change.ChangeType, test.ShouldEqual, pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED)
		_, err = stream.Next()
		test.That(t, err, test.ShouldEqual, io.EOF)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		stream = worldstatestore.NewTransformChangeStreamFromChannel(ctx, make(chan worldstatestore.TransformChange))
		_, err = stream.Next()
		test.That(t, err, test.ShouldBeError, context.Canceled)
	})
}
