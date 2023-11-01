package logging

import (
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
)

func TestNetLoggerQueueOperations(t *testing.T) {
	t.Run("test addBatchToQueue", func(t *testing.T) {
		queueSize := 10
		nl := NetAppender{
			maxQueueSize: queueSize,
		}

		nl.addBatchToQueue(make([]*apppb.LogEntry, queueSize-1))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize-1)

		nl.addBatchToQueue(make([]*apppb.LogEntry, 2))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)

		nl.addBatchToQueue(make([]*apppb.LogEntry, queueSize+1))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)
	})

	t.Run("test addToQueue", func(t *testing.T) {
		queueSize := 2
		nl := NetAppender{
			maxQueueSize: queueSize,
		}

		nl.addToQueue(&apppb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, 1)

		nl.addToQueue(&apppb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)

		nl.addToQueue(&apppb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)
	})
}
