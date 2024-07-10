package logging

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
)

func TestNetLoggerQueueOperations(t *testing.T) {
	t.Run("test addBatchToQueue", func(t *testing.T) {
		queueSize := 10
		nl := NetAppender{
			maxQueueSize: queueSize,
		}

		nl.addBatchToQueue(make([]*commonpb.LogEntry, queueSize-1))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize-1)

		nl.addBatchToQueue(make([]*commonpb.LogEntry, 2))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)

		nl.addBatchToQueue(make([]*commonpb.LogEntry, queueSize+1))
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)
	})

	t.Run("test addToQueue", func(t *testing.T) {
		queueSize := 2
		nl := NetAppender{
			maxQueueSize: queueSize,
		}

		nl.addToQueue(&commonpb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, 1)

		nl.addToQueue(&commonpb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)

		nl.addToQueue(&commonpb.LogEntry{})
		test.That(t, nl.queueSize(), test.ShouldEqual, queueSize)
	})
}

type mockRobotService struct {
	apppb.UnimplementedRobotServiceServer
	expectedID string

	logsMu              sync.Mutex
	logFailForSizeCount int
	logs                []*commonpb.LogEntry
	logBatches          [][]*commonpb.LogEntry
}

func (ms *mockRobotService) Log(ctx context.Context, req *apppb.LogRequest) (*apppb.LogResponse, error) {
	if ms.expectedID != req.Id {
		return nil, fmt.Errorf("expected id %q but got %q", ms.expectedID, req.Id)
	}
	ms.logsMu.Lock()
	defer ms.logsMu.Unlock()
	if ms.logFailForSizeCount > 0 {
		logsLeft := ms.logFailForSizeCount
		ms.logFailForSizeCount -= len(req.Logs)
		return &apppb.LogResponse{}, fmt.Errorf("not right now, %d log(s) left", logsLeft)
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

func TestNetLoggerSync(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	// This test is testing the behavior of sync(), so the background worker shouldn't be running at the same time.
	netAppender, err := newNetAppender(server.cloudConfig, nil, false, false)
	test.That(t, err, test.ShouldBeNil)

	logger := NewDebugLogger("test logger")
	// The stdout appender is not necessary for test correctness. But it does provide information in
	// the output w.r.t the injected grpc errors.
	logger.AddAppender(netAppender)

	for i := 0; i < writeBatchSize+1; i++ {
		logger.Infof("Some-info %d", i)
	}

	test.That(t, netAppender.sync(), test.ShouldBeNil)
	netAppender.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logBatches, test.ShouldHaveLength, 2)
	test.That(t, server.service.logBatches[0], test.ShouldHaveLength, 100)
	test.That(t, server.service.logBatches[1], test.ShouldHaveLength, 1)
	for i := 0; i < writeBatchSize+1; i++ {
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, fmt.Sprintf("Some-info %d", i))
	}
}

func TestNetLoggerSyncFailureAndRetry(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	// This test is testing the behavior of sync(), so the background worker shouldn't be running at the same time.
	netAppender, err := newNetAppender(server.cloudConfig, nil, false, false)
	test.That(t, err, test.ShouldBeNil)

	logger := NewDebugLogger("test logger")
	// The stdout appender is not necessary for test correctness. But it does provide information in
	// the output w.r.t the injected grpc errors.
	logger.AddAppender(netAppender)

	// This test will first log 10 "Some-info" logs. Followed by a single "New info" log.
	numLogs := 11

	// Inject a failure into the server handling `Log` requests.
	server.service.logsMu.Lock()
	server.service.logFailForSizeCount = numLogs
	server.service.logsMu.Unlock()

	for i := 0; i < numLogs-1; i++ {
		logger.Infof("Some-info %d", i)
	}

	// This test requires at least three syncs for the logs to be guaranteed received by the
	// server. Once the log queue is full of size ten batches, the first sync will decrement
	// `logFailForSizeCount` to 1 and return an error. The second will decrement it to a negative
	// value and return an error. The third sync will succeed.
	test.That(t, netAppender.sync(), test.ShouldNotBeNil)

	logger.Info("New info")

	test.That(t, netAppender.sync(), test.ShouldNotBeNil)
	test.That(t, netAppender.sync(), test.ShouldBeNil)

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, numLogs)
	for i := 0; i < numLogs-1; i++ {
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, fmt.Sprintf("Some-info %d", i))
	}
	test.That(t, server.service.logs[numLogs-1].Message, test.ShouldEqual, "New info")
}

func TestNetLoggerOverflowDuringWrite(t *testing.T) {
	// Lower defaultMaxQueueSize for test.
	originalDefaultMaxQueueSize := defaultMaxQueueSize
	defaultMaxQueueSize = 10
	defer func() {
		defaultMaxQueueSize = originalDefaultMaxQueueSize
	}()

	server := makeServerForRobotLogger(t)
	defer server.stop()

	netAppender, err := NewNetAppender(server.cloudConfig, nil, false)
	test.That(t, err, test.ShouldBeNil)
	logger := NewDebugLogger("test logger")
	logger.AddAppender(netAppender)

	// Lock server logsMu to mimic network latency for log syncing. Inject max
	// number of logs into netAppender queue. Wait for a Sync: syncOnce should
	// read the created, injected batch, send it to the server, and hang on
	// receiving a non-nil err.
	server.service.logsMu.Lock()
	for i := 0; i < defaultMaxQueueSize; i++ {
		netAppender.addToQueue(&commonpb.LogEntry{Message: fmt.Sprint(i)})
	}

	// Sleep to ensure syncOnce happens (normally every 100ms) and hangs in
	// receiving non-nil error from write to remote.
	time.Sleep(300 * time.Millisecond)

	// This "10" log should "overflow" the netAppender queue and remove the "0"
	// (oldest) log. syncOnce should sense that an overflow occurred and only
	// remove "1"-"9" from the queue.
	logger.Info("10")
	server.service.logsMu.Unlock()

	// Close net appender to cause final syncOnce that sends batch of logs after
	// overflow was accounted for: ["10"].
	netAppender.Close()

	// Server should have received logs with Messages: ["0", "1", "2", "3", "4",
	// "5", "6", "7", "8", "9", "10"].
	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, 11)
	for i := 0; i < 11; i++ {
		// First batch of "0"-"10".
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, fmt.Sprint(i))
	}
}

// TestProvidedClientConn tests non-nil `conn` param to NewNetAppender.
func TestProvidedClientConn(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()
	conn, err := CreateNewGRPCClient(context.Background(), server.cloudConfig)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()
	netAppender, err := NewNetAppender(server.cloudConfig, conn, true)
	test.That(t, err, test.ShouldBeNil)
	// make sure these are the same object, i.e. that the constructor set it properly.
	test.That(t, netAppender.remoteWriter.rpcClient == conn, test.ShouldBeTrue)
	test.That(t, netAppender.remoteWriter.service, test.ShouldNotBeNil)

	logger := NewDebugLogger("provided-client-conn")
	logger.AddAppender(netAppender)

	test.That(t, server.service.logs, test.ShouldBeEmpty)
	logger.Info("hello")
	netAppender.Close()
	test.That(t, server.service.logs, test.ShouldHaveLength, 1)
}

func TestSetConn(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	// when inheritConn=true, getOrCreateClient should return uninitializedConnectionError
	netAppender, err := NewNetAppender(server.cloudConfig, nil, true)
	test.That(t, err, test.ShouldBeNil)
	client, err := netAppender.remoteWriter.getOrCreateClient(context.Background())
	test.That(t, client, test.ShouldBeNil)
	test.That(t, errors.Is(err, errUninitializedConnection), test.ShouldBeTrue)

	// write a line before the connection is up
	logger := NewDebugLogger("provided-client-conn")
	logger.AddAppender(netAppender)
	logger.Info("pre-connect")

	// now set a connection
	conn, err := CreateNewGRPCClient(context.Background(), server.cloudConfig)
	test.That(t, err, test.ShouldBeNil)
	netAppender.SetConn(conn, true)
	test.That(t, server.service.logs, test.ShouldBeEmpty)

	// and log, and make sure both lines sync
	logger.Info("post-connect")
	netAppender.Close()
	test.That(t, server.service.logs, test.ShouldHaveLength, 2)
}

// construct a NetAppender for testing with no background runners.
func quickFakeAppender(t *testing.T) *NetAppender {
	t.Helper()
	return &NetAppender{
		toLog:            make([]*commonpb.LogEntry, 0),
		remoteWriter:     &remoteLogWriterGRPC{},
		cancel:           func() {},
		loggerWithoutNet: NewTestLogger(t),
	}
}

func TestNetAppenderClose(t *testing.T) {
	totalIters := 100
	exitIters := 10

	t.Run("progress", func(t *testing.T) {
		na := quickFakeAppender(t)
		for i := 0; i < totalIters; i++ {
			na.toLog = append(na.toLog, &commonpb.LogEntry{})
		}
		iters := 0
		na.close(exitIters, totalIters, func(time.Duration) {
			iters++
			na.toLog = na.toLog[1:]
		})
		test.That(t, iters, test.ShouldEqual, totalIters)
	})

	t.Run("no-progress", func(t *testing.T) {
		na := quickFakeAppender(t)
		na.toLog = append(na.toLog, &commonpb.LogEntry{})
		iters := 0
		na.close(exitIters, totalIters, func(time.Duration) {
			iters++
		})
		test.That(t, iters, test.ShouldEqual, exitIters)
	})
}
