package logging

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
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

type mockRobotService struct {
	apppb.UnimplementedRobotServiceServer
	expectedID string

	logsMu              sync.Mutex
	logFailForSizeCount int
	logs                []*apppb.LogEntry
	logBatches          [][]*apppb.LogEntry
}

func (ms *mockRobotService) Log(ctx context.Context, req *apppb.LogRequest) (*apppb.LogResponse, error) {
	if ms.expectedID != req.Id {
		return nil, fmt.Errorf("expected id %q but got %q", ms.expectedID, req.Id)
	}
	ms.logsMu.Lock()
	defer ms.logsMu.Unlock()
	if ms.logFailForSizeCount > 0 {
		ms.logFailForSizeCount -= len(req.Logs)
		return &apppb.LogResponse{}, errors.New("not right now")
	}
	ms.logs = append(ms.logs, req.Logs...)
	ms.logBatches = append(ms.logBatches, req.Logs)
	return &apppb.LogResponse{}, nil
}

type serverForRobotLogger struct {
	service     *mockRobotService
	cloudConfig *CloudConfig
	stop        func() error
}

func makeServerForRobotLogger(t *testing.T) serverForRobotLogger {
	logger := NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger.AsZap(), rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	robotService := &mockRobotService{expectedID: "abc-123"}
	test.That(t, rpcServer.RegisterServiceServer(
		context.Background(),
		&apppb.RobotService_ServiceDesc,
		robotService,
		apppb.RegisterRobotServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	config := &CloudConfig{
		AppAddress: fmt.Sprintf("http://%s", listener.Addr().String()),
		ID:         robotService.expectedID,
	}
	return serverForRobotLogger{robotService, config, rpcServer.Stop}
}

func TestNetLoggerBatchWrites(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	netAppender, err := NewNetAppender(server.cloudConfig)
	test.That(t, err, test.ShouldBeNil)

	logger := NewViamLogger("test logger")
	// The stdout appender is not necessary for test correctness. But it does provide information in
	// the output w.r.t the injected grpc errors.
	logger.AddAppender(NewStdoutAppender())
	logger.AddAppender(netAppender)

	for i := 0; i < writeBatchSize+1; i++ {
		logger.Info("Some-info")
	}

	netAppender.Sync()
	netAppender.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logBatches, test.ShouldHaveLength, 2)
	test.That(t, server.service.logBatches[0], test.ShouldHaveLength, 100)
	test.That(t, server.service.logBatches[1], test.ShouldHaveLength, 1)
	for i := 0; i < writeBatchSize+1; i++ {
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, "Some-info")
	}
}

func TestNetLoggerBatchFailureAndRetry(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	netAppender, err := NewNetAppender(server.cloudConfig)
	test.That(t, err, test.ShouldBeNil)
	logger := NewViamLogger("test logger")
	// The stdout appender is not necessary for test correctness. But it does provide information in
	// the output w.r.t the injected grpc errors.
	logger.AddAppender(NewStdoutAppender())
	logger.AddAppender(netAppender)

	// This test will first log 10 "Some-info" logs. Followed by a single "New info" log.
	numLogs := 11

	// Injet a failure into the server handling `Log` requests.
	server.service.logsMu.Lock()
	server.service.logFailForSizeCount = numLogs
	server.service.logsMu.Unlock()

	for i := 0; i < numLogs-1; i++ {
		logger.Info("Some-info")
	}

	// This test requires at least three syncs for the logs to be guaranteed received by the
	// server. Once the log queue is full of size ten batches, the first sync will decrement
	// `logFailForSizeCount` to 1 and return an error. The second will decrement it to a negative
	// value and return an error. The third will succeed.
	//
	// This test depends on the `Close` method performing a `Sync`.
	//
	// The `netAppender` also has a background worker syncing on its own cadence. This complicates
	// exactly which syncs do what work and which ones return errors.
	netAppender.Sync()

	logger.Info("New info")

	netAppender.Sync()
	netAppender.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, numLogs)
	for i := 0; i < numLogs-1; i++ {
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, "Some-info")
	}
	test.That(t, server.service.logs[numLogs-1].Message, test.ShouldEqual, "New info")
}
