package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

func TestNetLoggerQueueOperations(t *testing.T) {
	t.Run("test addBatchToQueue", func(t *testing.T) {
		queueSize := 10
		nl := netLogger{
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
		nl := netLogger{
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
	cloudConfig *config.Cloud
	stop        func() error
}

func makeServerForRobotLogger(t *testing.T) serverForRobotLogger {
	logger := logging.NewTestLogger(t)
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
	config := &config.Cloud{
		AppAddress: fmt.Sprintf("http://%s", listener.Addr().String()),
		ID:         robotService.expectedID,
	}
	return serverForRobotLogger{robotService, config, rpcServer.Stop}
}

func TestNetLoggerBatchWrites(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	logger := logging.NewTestLogger(t)
	nl, err := newNetLogger(server.cloudConfig, logger, zap.NewAtomicLevelAt(zap.InfoLevel))
	test.That(t, err, test.ShouldBeNil)

	loggerWithNet := logger.Desugar()
	loggerWithNet = loggerWithNet.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	for i := 0; i < writeBatchSize+1; i++ {
		loggerWithNet.Info("Some-info")
	}

	nl.Sync()
	nl.Close()

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

	logger := logging.NewTestLogger(t)
	nl, err := newNetLogger(server.cloudConfig, logger, zap.NewAtomicLevelAt(zap.InfoLevel))
	test.That(t, err, test.ShouldBeNil)

	loggerWithNet := logger.Desugar()
	loggerWithNet = loggerWithNet.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	numLogs := 11
	server.service.logsMu.Lock()
	server.service.logFailForSizeCount = 11
	server.service.logsMu.Unlock()

	for i := 0; i < numLogs-1; i++ {
		loggerWithNet.Info("Some-info")
	}
	nl.Sync()

	loggerWithNet.Info("New info")

	nl.Sync()
	nl.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, numLogs)
	for i := 0; i < numLogs-1; i++ {
		test.That(t, server.service.logs[i].Message, test.ShouldEqual, "Some-info")
	}
	test.That(t, server.service.logs[numLogs-1].Message, test.ShouldEqual, "New info")
}

func TestNetLoggerUnderlyingLoggerDoesntRecurse(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	logger := logging.NewTestLogger(t)
	nl, err := newNetLogger(server.cloudConfig, logger, zap.NewAtomicLevelAt(zap.InfoLevel))
	test.That(t, err, test.ShouldBeNil)

	loggerWithNet := logger.Desugar()
	loggerWithNet = loggerWithNet.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	loggerWithNet.Info("should write to network")
	logger.Info("should not write to network")

	nl.Sync()
	nl.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, 1)
	test.That(t, server.service.logs[0].Message, test.ShouldEqual, "should write to network")
}

func TestNetLoggerLogLevel(t *testing.T) {
	server := makeServerForRobotLogger(t)
	defer server.stop()

	logger := logging.NewTestLogger(t)
	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	nl, err := newNetLogger(server.cloudConfig, logger, level)
	test.That(t, err, test.ShouldBeNil)

	loggerWithNet := logger.Desugar()
	loggerWithNet = loggerWithNet.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	loggerWithNet.Info("info level")
	loggerWithNet.Debug("debug level")
	nl.Sync()

	level.SetLevel(zap.DebugLevel)

	loggerWithNet.Info("info level")
	loggerWithNet.Debug("debug level")
	nl.Sync()

	nl.Close()

	server.service.logsMu.Lock()
	defer server.service.logsMu.Unlock()
	test.That(t, server.service.logs, test.ShouldHaveLength, 3)
	test.That(t, server.service.logs[0].Message, test.ShouldEqual, "info level")
	test.That(t, server.service.logs[0].Level, test.ShouldEqual, "info")
	test.That(t, server.service.logs[1].Message, test.ShouldEqual, "info level")
	test.That(t, server.service.logs[1].Level, test.ShouldEqual, "info")
	test.That(t, server.service.logs[2].Message, test.ShouldEqual, "debug level")
	test.That(t, server.service.logs[2].Level, test.ShouldEqual, "debug")
}
