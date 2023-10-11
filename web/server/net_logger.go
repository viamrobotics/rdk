// Package server implements the entry point for running a robot web server.
package server

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

const (
	defaultMaxQueueSize = 20000
	writeBatchSize      = 100
)

type wrappedLogger struct {
	base  zapcore.Core
	extra []zapcore.Field
}

func (l *wrappedLogger) Enabled(level zapcore.Level) bool {
	return l.base.Enabled(level)
}

func (l *wrappedLogger) With(f []zapcore.Field) zapcore.Core {
	return &wrappedLogger{l, f}
}

func (l *wrappedLogger) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return l.base.Check(e, ce)
}

func (l *wrappedLogger) Write(e zapcore.Entry, f []zapcore.Field) error {
	field := []zapcore.Field{}
	field = append(field, l.extra...)
	field = append(field, f...)
	return l.base.Write(e, field)
}

func (l *wrappedLogger) Sync() error {
	return l.base.Sync()
}

func newNetLogger(config *config.Cloud, loggerWithoutNet logging.Logger, logLevel zap.AtomicLevel) (*netLogger, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	logWriter := &remoteLogWriterGRPC{
		loggerWithoutNet: loggerWithoutNet,
		cfg:              config,
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	nl := &netLogger{
		hostname:         hostname,
		cancelCtx:        cancelCtx,
		cancel:           cancel,
		remoteWriter:     logWriter,
		maxQueueSize:     defaultMaxQueueSize,
		loggerWithoutNet: loggerWithoutNet,
		logLevel:         logLevel,
	}
	nl.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(nl.backgroundWorker, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

type netLogger struct {
	hostname     string
	remoteWriter remoteLogWriter

	toLogMutex   sync.Mutex
	toLog        []*apppb.LogEntry
	maxQueueSize int

	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup

	// Use this logger for library errors that will not be reported through
	// the netLogger causing a recursive loop.
	loggerWithoutNet logging.Logger

	// Log level of the rdk system
	logLevel zap.AtomicLevel
}

func (nl *netLogger) queueSize() int {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()
	return len(nl.toLog)
}

func (nl *netLogger) Close() {
	// try for up to 10 seconds for log queue to clear before cancelling it
	for i := 0; i < 1000; i++ {
		if nl.queueSize() == 0 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}
	nl.cancel()
	nl.activeBackgroundWorkers.Wait()
	nl.remoteWriter.close()
}

func (nl *netLogger) Enabled(l zapcore.Level) bool {
	return nl.logLevel.Enabled(l)
}

func (nl *netLogger) With(f []zapcore.Field) zapcore.Core {
	return &wrappedLogger{nl, f}
}

func (nl *netLogger) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if nl.logLevel.Enabled(e.Level) {
		return ce.AddCore(e, nl)
	}
	return ce
}

// Mirrors zapcore.EntryCaller but leaves out the pointer address.
type wrappedEntryCaller struct {
	Defined  bool
	File     string
	Line     int
	Function string
}

func (nl *netLogger) Write(e zapcore.Entry, f []zapcore.Field) error {
	// TODO(erh): should we put a _id uuid on here so we don't log twice?

	log := &apppb.LogEntry{
		Host:       nl.hostname,
		Level:      e.Level.String(),
		Time:       timestamppb.New(e.Time),
		LoggerName: e.LoggerName,
		Message:    e.Message,
		Stack:      e.Stack,
	}

	wc := wrappedEntryCaller{
		Defined:  e.Caller.Defined,
		File:     e.Caller.File,
		Line:     e.Caller.Line,
		Function: e.Caller.Function,
	}

	caller, err := protoutils.StructToStructPb(wc)
	if err != nil {
		return err
	}
	log.Caller = caller

	fields := make([]*structpb.Struct, 0, len(f))
	for _, ff := range f {
		if ff.String == "" && ff.Interface != nil {
			ff.String = fmt.Sprintf("%v", ff.Interface)
		}

		field, err := protoutils.StructToStructPb(ff)
		if err != nil {
			return err
		}

		fields = append(fields, field)
	}
	log.Fields = fields

	nl.addToQueue(log)

	if e.Level == zapcore.FatalLevel || e.Level == zapcore.DPanicLevel || e.Level == zapcore.PanicLevel {
		// program is going to go away, let's try and sync all our messages before then
		return nl.Sync()
	}

	return nil
}

func (nl *netLogger) addToQueue(x *apppb.LogEntry) {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(nl.toLog) >= nl.maxQueueSize {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[1:]
	}
	nl.toLog = append(nl.toLog, x)
}

func (nl *netLogger) addBatchToQueue(x []*apppb.LogEntry) {
	if len(x) == 0 {
		return
	}

	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(x) > nl.maxQueueSize {
		x = x[len(x)-nl.maxQueueSize:]
	}

	if len(nl.toLog)+len(x) >= nl.maxQueueSize {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[len(nl.toLog)+len(x)-nl.maxQueueSize:]
	}

	nl.toLog = append(nl.toLog, x...)
}

func (nl *netLogger) backgroundWorker() {
	normalInterval := 100 * time.Millisecond
	abnormalInterval := 5 * time.Second
	interval := normalInterval
	for {
		cancelled := false
		if !utils.SelectContextOrWait(nl.cancelCtx, interval) {
			cancelled = true
		}
		err := nl.Sync()
		if err != nil && !errors.Is(err, context.Canceled) {
			interval = abnormalInterval
			nl.loggerWithoutNet.Infof("error logging to network: %s", err)
		} else {
			interval = normalInterval
		}
		if cancelled {
			return
		}
	}
}

func (nl *netLogger) Sync() error {
	for {
		x := func() []*apppb.LogEntry {
			nl.toLogMutex.Lock()
			defer nl.toLogMutex.Unlock()

			if len(nl.toLog) == 0 {
				return nil
			}

			batchSize := writeBatchSize
			if len(nl.toLog) < writeBatchSize {
				batchSize = len(nl.toLog)
			}

			x := nl.toLog[:batchSize]
			nl.toLog = nl.toLog[batchSize:]

			return x
		}()

		if len(x) == 0 {
			return nil
		}

		err := nl.remoteWriter.write(x)
		if err != nil {
			nl.addBatchToQueue(x)
			return err
		}
	}
}

func addCloudLogger(logger logging.Logger, logLevel zap.AtomicLevel, cfg *config.Cloud) (golog.Logger, func(), error) {
	nl, err := newNetLogger(cfg, logger, logLevel)
	if err != nil {
		return nil, nil, err
	}
	l := logger.Desugar()
	l = l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))
	return l.Sugar(), nl.Close, nil
}

type remoteLogWriter interface {
	write(logs []*apppb.LogEntry) error
	close()
}

type remoteLogWriterGRPC struct {
	cfg *config.Cloud

	// `service` and `rpcClient` are lazily initialized on first call to `write`. The `clientMutex`
	// serializes access to ensure that only one caller creates them.
	service     apppb.RobotServiceClient
	rpcClient   rpc.ClientConn
	clientMutex sync.Mutex

	// Use this logger for library errors that will not be reported through
	// the netLogger causing a recursive loop.
	loggerWithoutNet logging.Logger
}

func (w *remoteLogWriterGRPC) write(logs []*apppb.LogEntry) error {
	// we specifically don't use a parented cancellable context here so we can make sure we finish writing but
	// we will only give it up to 5 seconds to do so in case we are trying to shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	client, err := w.getOrCreateClient(ctx)
	if err != nil {
		return err
	}

	_, err = client.Log(ctx, &apppb.LogRequest{Id: w.cfg.ID, Logs: logs})
	if err != nil {
		return err
	}

	return nil
}

func (w *remoteLogWriterGRPC) getOrCreateClient(ctx context.Context) (apppb.RobotServiceClient, error) {
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()

	if w.service != nil {
		return w.service, nil
	}

	client, err := config.CreateNewGRPCClient(ctx, w.cfg, w.loggerWithoutNet)
	if err != nil {
		return nil, err
	}

	w.rpcClient = client
	w.service = apppb.NewRobotServiceClient(w.rpcClient)
	return w.service, nil
}

func (w *remoteLogWriterGRPC) close() {
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()

	if w.rpcClient != nil {
		utils.UncheckedErrorFunc(w.rpcClient.Close)
	}
}
