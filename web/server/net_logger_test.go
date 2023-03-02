package server

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	mockapppb "go.viam.com/api/app/mock_v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestNewNetLogger(t *testing.T) {
	logger := golog.NewTestLogger(t)
	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	nl, err := newNetLogger(&config.Cloud{
		AppAddress: "http://localhost:8080",
		ID:         "abc-123",
	}, logger, level)
	test.That(t, err, test.ShouldBeNil)

	_, ok := nl.remoteWriter.(*remoteLogWriterGRPC)
	test.That(t, ok, test.ShouldBeTrue)
	nl.cancel()
}

func TestNewNetBatchWrites(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockapppb.NewMockRobotServiceClient(ctrl)

	logger := golog.NewTestLogger(t)
	cancelCtx, cancel := context.WithCancel(context.Background())

	config := &config.Cloud{
		AppAddress: "http://localhost:8080",
		ID:         "abc-123",
	}

	logWriter := &remoteLogWriterGRPC{
		loggerWithoutNet: logger,
		cfg:              config,
		service:          client,
	}

	nl := &netLogger{
		hostname:     "hostname",
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		remoteWriter: logWriter,
		maxQueueSize: 1000,
		logLevel:     zap.NewAtomicLevelAt(zap.InfoLevel),
	}

	l := logger.Desugar()
	l = l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 100, id: "abc-123"}).
		Times(1).
		Return(&apppb.LogResponse{}, nil)

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 1, id: "abc-123"}).
		Times(1).
		Return(&apppb.LogResponse{}, nil)

	for i := 0; i < writeBatchSize+1; i++ {
		l.Info("Some-info")
	}

	nl.Sync()
	nl.Close()
}

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

func TestBatchFailureAndRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockapppb.NewMockRobotServiceClient(ctrl)

	logger := golog.NewTestLogger(t)
	cancelCtx, cancel := context.WithCancel(context.Background())

	config := &config.Cloud{
		AppAddress: "http://localhost:8080",
		ID:         "abc-123",
	}

	logWriter := &remoteLogWriterGRPC{
		loggerWithoutNet: logger,
		cfg:              config,
		service:          client,
	}

	nl := &netLogger{
		hostname:     "hostname",
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		remoteWriter: logWriter,
		maxQueueSize: 100,
		logLevel:     zap.NewAtomicLevelAt(zap.InfoLevel),
	}

	l := logger.Desugar()
	l = l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 10, id: "abc-123"}).
		Times(1).
		Return(nil, errors.New("failure"))

	for i := 0; i < 10; i++ {
		l.Info("Some-info")
	}
	nl.Sync()

	l.Info("New info")
	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 11, id: "abc-123"}).
		Times(1).
		Return(&apppb.LogResponse{}, nil)

	nl.Sync()

	nl.Close()
}

func TestUnderlyingLoggerDoesntRecurse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockapppb.NewMockRobotServiceClient(ctrl)

	logger := golog.NewTestLogger(t)
	cancelCtx, cancel := context.WithCancel(context.Background())

	config := &config.Cloud{
		AppAddress: "http://localhost:8080",
		ID:         "abc-123",
	}

	logWriter := &remoteLogWriterGRPC{
		loggerWithoutNet: logger,
		cfg:              config,
		service:          client,
	}

	nl := &netLogger{
		hostname:         "hostname",
		cancelCtx:        cancelCtx,
		cancel:           cancel,
		remoteWriter:     logWriter,
		maxQueueSize:     100,
		loggerWithoutNet: logger,
		logLevel:         zap.NewAtomicLevelAt(zap.InfoLevel),
	}

	newLogger := logger.Desugar()
	newLogger = newLogger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 1, id: "abc-123"}).
		Times(1).
		Return(nil, errors.New("failure"))

	newLogger.Info("should write to network")
	logger.Info("should not write to network")
	nl.Sync()

	nl.Close()
}

func TestLogLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mockapppb.NewMockRobotServiceClient(ctrl)

	logger := golog.NewTestLogger(t)
	cancelCtx, cancel := context.WithCancel(context.Background())

	config := &config.Cloud{
		AppAddress: "http://localhost:8080",
		ID:         "abc-123",
	}

	logWriter := &remoteLogWriterGRPC{
		loggerWithoutNet: logger,
		cfg:              config,
		service:          client,
	}

	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	nl := &netLogger{
		hostname:         "hostname",
		cancelCtx:        cancelCtx,
		cancel:           cancel,
		remoteWriter:     logWriter,
		maxQueueSize:     100,
		loggerWithoutNet: logger,
		logLevel:         level,
	}

	newLogger := logger.Desugar()
	newLogger = newLogger.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 1, id: "abc-123"}).
		Times(1).
		Return(&apppb.LogResponse{}, nil)

	newLogger.Info("info level")
	newLogger.Debug("debug level")
	nl.Sync()

	level.SetLevel(zap.DebugLevel)

	client.EXPECT().
		Log(gomock.Any(), &expectedLogMatch{nLogs: 2, id: "abc-123"}).
		Times(1).
		Return(&apppb.LogResponse{}, nil)

	newLogger.Info("info level")
	newLogger.Debug("debug level")
	nl.Sync()

	nl.Close()
}

type expectedLogMatch struct {
	nLogs int
	id    string
}

func (lr *expectedLogMatch) Matches(x interface{}) bool {
	m, ok := x.(*apppb.LogRequest)
	if !ok {
		return false
	}

	if m.Id != lr.id {
		return false
	}

	if len(m.Logs) != lr.nLogs {
		return false
	}

	return true
}

func (lr *expectedLogMatch) String() string {
	return fmt.Sprintf("Expected LogRequest with %d logs", &lr.nLogs)
}
