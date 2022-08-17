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
	mockapppb "go.viam.com/api/proto/viam/app/mock_v1"
	apppb "go.viam.com/api/proto/viam/app/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestNewNetLogger(t *testing.T) {
	t.Run("with AppConfig should use GRPC", func(t *testing.T) {
		nl, err := newNetLogger(&config.Cloud{
			AppAddress: "http://localhost:8080",
			ID:         "abc-123",
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := nl.remoteWriter.(*remoteLogWriterGRPC)
		test.That(t, ok, test.ShouldBeTrue)
		nl.cancel()
	})

	t.Run("with AppConfig should use HTTP", func(t *testing.T) {
		nl, err := newNetLogger(&config.Cloud{
			LogPath: "http://localhost:8080/logs",
			ID:      "abc-123",
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := nl.remoteWriter.(*remoteLogWriterHTTP)
		test.That(t, ok, test.ShouldBeTrue)
		nl.cancel()
	})
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
		logger:  logger,
		cfg:     config,
		service: client,
	}

	nl := &netLogger{
		hostname:     "hostname",
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		remoteWriter: logWriter,
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
		logger:  logger,
		cfg:     config,
		service: client,
	}

	nl := &netLogger{
		hostname:     "hostname",
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		remoteWriter: logWriter,
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
