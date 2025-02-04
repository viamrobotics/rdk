package logging

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	apppb "go.viam.com/api/app/v1"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	defaultMaxQueueSize        = 20000
	writeBatchSize             = 100
	errUninitializedConnection = errors.New("sharedConn is true and connection is not initialized")
	logWriteTimeout            = 4 * time.Second
	logWriteTimeoutBehindProxy = time.Minute
)

// CloudConfig contains the necessary inputs to send logs to the app backend over grpc.
type CloudConfig struct {
	AppAddress string
	ID         string
	Secret     string
}

// NewNetAppender creates a NetAppender to send log events to the app backend. NetAppenders ought to
// be `Close`d prior to shutdown to flush remaining logs.
// Pass `nil` for `conn` if you want this to create its own connection.
func NewNetAppender(
	config *CloudConfig,
	conn rpc.ClientConn,
	sharedConn bool,
	loggerWithoutNet Logger,
) (*NetAppender, error) {
	return newNetAppender(config, conn, sharedConn, true, loggerWithoutNet)
}

// inner function for NewNetAppender which can disable background worker in tests.
func newNetAppender(
	config *CloudConfig,
	conn rpc.ClientConn,
	sharedConn,
	startBackgroundWorker bool,
	loggerWithoutNet Logger,
) (*NetAppender, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	logWriter := &remoteLogWriterGRPC{
		cfg:              config,
		sharedConn:       sharedConn,
		loggerWithoutNet: loggerWithoutNet,
	}

	cancelCtx, cancel := context.WithCancel(context.Background())

	nl := &NetAppender{
		hostname:         hostname,
		cancelCtx:        cancelCtx,
		cancel:           cancel,
		remoteWriter:     logWriter,
		maxQueueSize:     defaultMaxQueueSize,
		loggerWithoutNet: loggerWithoutNet,
	}

	nl.SetConn(conn, sharedConn)

	if startBackgroundWorker {
		nl.activeBackgroundWorkers.Add(1)
		utils.ManagedGo(nl.backgroundWorker, nl.activeBackgroundWorkers.Done)
	}
	return nl, nil
}

// NetAppender can send log events to the app backend.
type NetAppender struct {
	hostname     string
	remoteWriter *remoteLogWriterGRPC

	// toLogMutex guards toLog and toLogOverflowsSinceLastSync.
	toLogMutex                  sync.Mutex
	toLog                       []*commonpb.LogEntry
	toLogOverflowsSinceLastSync int

	maxQueueSize int

	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup

	// `loggerWithoutNet` is the logger to use for meta/internal logs
	// from the `NetAppender`.
	loggerWithoutNet Logger
}

func (w *remoteLogWriterGRPC) setConn(ctx context.Context, logger Logger, conn rpc.ClientConn, sharedConn bool) {
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()
	if w.rpcClient != nil && !w.sharedConn {
		oldClient := w.rpcClient
		err := oldClient.Close()
		logger.CErrorf(ctx, "error closing oldClient: %s", err)
	}
	w.rpcClient = conn
	w.service = nil
	if conn != nil {
		w.service = apppb.NewRobotServiceClient(conn)
	}
	// note: this is tricky. It means that if you ever call setConn, sharedConn is true for all time.
	w.sharedConn = sharedConn
}

// SetConn sets the GRPC connection used by the NetAppender.
// sharedConn should be true in all external calls to this. If sharedConn=false, the NetAppender will close
// the connection in Close(). conn may be nil.
func (nl *NetAppender) SetConn(conn rpc.ClientConn, sharedConn bool) {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()
	nl.remoteWriter.setConn(nl.cancelCtx, nl.loggerWithoutNet, conn, sharedConn)
}

func (nl *NetAppender) queueSize() int {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()
	return len(nl.toLog)
}

// cancelBackgroundWorkers is an internal function meant to be used only in testing and in Close(). NetAppender will
// not function correctly if cancelBackgroundWorkers() is called outside of those contexts.
func (nl *NetAppender) cancelBackgroundWorkers() {
	if nl.cancel != nil {
		nl.cancel()
		nl.cancel = nil
	}
	nl.activeBackgroundWorkers.Wait()
}

// Close the NetAppender. This makes a best effort at sending all logs before returning.
func (nl *NetAppender) Close() {
	nl.close(150, 1000, func(durt time.Duration) { time.Sleep(durt) })
}

// The inner close() can take a mocked sleep function for testing.
// `exitIfNoProgressIters` is a stopping condition; if this many iters pass without the log
// queue shrinking, we break the loop. This behavior prevents slow shutdown when offline.

// `totalIters` is the longest possible wait time.
// `sleepFn` is called between every iter.
func (nl *NetAppender) close(exitIfNoProgressIters, totalIters int, sleepFn func(durt time.Duration)) {
	prevQueue := nl.queueSize()
	lastProgressIter := 0
	sleepInterval := 10 * time.Millisecond
	if nl.cancel != nil {
		// try for up to 10 seconds for log queue to clear before cancelling it
		for i := 0; i < totalIters; i++ {
			curQueue := nl.queueSize()
			// A batch can be popped from the queue for a sync by the background worker, and re-enqueued
			// due to an error. This check does not account for this case. It will cancel the background
			// worker once the last batch is in flight. Successful or not.
			if curQueue == 0 {
				break
			}
			if curQueue < prevQueue {
				prevQueue = curQueue
				lastProgressIter = i
			}
			if i-lastProgressIter >= exitIfNoProgressIters {
				nl.loggerWithoutNet.Warnf("NetAppender.Close() did not progress in %s, closing with %d still in queue",
					time.Duration(exitIfNoProgressIters)*sleepInterval, curQueue)
				break
			}
			sleepFn(sleepInterval)
		}
	}
	nl.cancelBackgroundWorkers()
	nl.remoteWriter.close()
}

// Mirrors zapcore.EntryCaller but leaves out the pointer address.
type wrappedEntryCaller struct {
	Defined  bool
	File     string
	Line     int
	Function string
}

func (nl *NetAppender) Write(e zapcore.Entry, f []zapcore.Field) error {
	log := &commonpb.LogEntry{
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
		return nl.sync()
	}

	return nil
}

// addToQueue adds a LogEntry to the net appender's queue, discarding the
// oldest entry in the queue if the size of the queue has overflowed.
func (nl *NetAppender) addToQueue(logEntry *commonpb.LogEntry) {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(nl.toLog) >= nl.maxQueueSize {
		// TODO(RSDK-7000): Selectively kick logs out of the queue based on log
		// content (i.e. don't maintain 20000 trivial logs of "Foo" when other,
		// potentially important information is being logged).
		nl.toLog = nl.toLog[1:]
		nl.toLogOverflowsSinceLastSync++
	}
	nl.toLog = append(nl.toLog, logEntry)
}

// addBatchToQueue adds a slice of LogEntrys to the net appender's queue,
// trimming the front of the batch to be less than maxQueueSize, and discarding
// the oldest len(batch) entries in the queue if the queue has overflowed.
func (nl *NetAppender) addBatchToQueue(batch []*commonpb.LogEntry) {
	if len(batch) == 0 {
		return
	}

	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(batch) > nl.maxQueueSize {
		batch = batch[len(batch)-nl.maxQueueSize:]
	}

	if len(nl.toLog)+len(batch) >= nl.maxQueueSize {
		// TODO(RSDK-7000): Selectively kick logs out of the queue based on log
		// content (i.e. don't maintain 20000 trivial logs of "Foo" when other,
		// potentially important information is being logged).
		overflow := len(nl.toLog) + len(batch) - nl.maxQueueSize
		nl.toLog = nl.toLog[overflow:]
		nl.toLogOverflowsSinceLastSync += overflow
	}

	nl.toLog = append(nl.toLog, batch...)
}

func (nl *NetAppender) backgroundWorker() {
	normalInterval := 100 * time.Millisecond
	abnormalInterval := 5 * time.Second
	interval := normalInterval
	for {
		if !utils.SelectContextOrWait(nl.cancelCtx, interval) {
			return
		}
		err := nl.sync()
		if err != nil && !errors.Is(err, context.Canceled) {
			interval = abnormalInterval
			if !errors.Is(err, errUninitializedConnection) {
				nl.loggerWithoutNet.Infof("error logging to network: %s", err)
			}
		} else {
			interval = normalInterval
		}
	}
}

// Returns whether there is more work to do or if an error was encountered
// while trying to ship logs over the network.
func (nl *NetAppender) syncOnce() (bool, error) {
	nl.toLogMutex.Lock()

	if len(nl.toLog) == 0 {
		nl.toLogMutex.Unlock()
		return false, nil
	}

	batchSize := writeBatchSize
	if len(nl.toLog) < writeBatchSize {
		batchSize = len(nl.toLog)
	}

	// Read a batch from the queue, unlock mutex, and return an error if write
	// fails. Lock mutex again to remove batch from queue only if write succeeded
	// and front of queue was not mutated by addToQueue/addBatchToQueue throwing
	// away the oldest logs due to overflows beyond maxQueueSize.
	batch := nl.toLog[:batchSize]
	nl.toLogMutex.Unlock()

	if err := nl.remoteWriter.write(nl.cancelCtx, batch); err != nil {
		return false, err
	}

	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	// Remove successfully synced logs from the queue. If we've overflowed more times than the size of the batch
	// we wrote, do not mutate toLog at all. If we've synced more logs than there are logs left, set idx to length
	// of array to prevent panics.
	if batchSize > nl.toLogOverflowsSinceLastSync {
		idx := min(batchSize-nl.toLogOverflowsSinceLastSync, len(nl.toLog))
		nl.toLog = nl.toLog[idx:]
	}
	nl.toLogOverflowsSinceLastSync = 0
	return len(nl.toLog) > 0, nil
}

// sync will flush the internal buffer of logs. This is not exposed as multiple calls to sync at
// the same time will cause double logs and panics.
func (nl *NetAppender) sync() error {
	for {
		moreToDo, err := nl.syncOnce()
		if err != nil {
			return err
		}

		if !moreToDo {
			return nil
		}
	}
}

// Sync is a no-op. sync is not exposed as multiple calls at the same time will cause double logs and panics.
func (nl *NetAppender) Sync() error {
	return nil
}

type remoteLogWriterGRPC struct {
	cfg *CloudConfig

	// `service` and `rpcClient` are lazily initialized on first call to `write`. The `clientMutex`
	// serializes access to ensure that only one caller creates them.
	service     apppb.RobotServiceClient
	rpcClient   rpc.ClientConn
	clientMutex sync.Mutex
	// When sharedConn = true, don't create or destroy connections; use what we're given.
	sharedConn bool

	// `loggerWithoutNet` is the logger to use for meta/internal logs from the `remoteLogWriterGRPC`.
	loggerWithoutNet Logger
}

func (w *remoteLogWriterGRPC) write(ctx context.Context, logs []*commonpb.LogEntry) error {
	timeout := logWriteTimeout
	// When environment indicates we are behind a proxy, bump timeout. Network
	// operations tend to take longer when behind a proxy.
	if proxyAddr := os.Getenv(rpc.SocksProxyEnvVar); proxyAddr != "" {
		timeout = logWriteTimeoutBehindProxy
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
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

	if w.sharedConn {
		return nil, errUninitializedConnection
	}

	client, err := CreateNewGRPCClient(ctx, w.cfg, w.loggerWithoutNet)
	if err != nil {
		return nil, err
	}

	w.rpcClient = client
	w.service = apppb.NewRobotServiceClient(w.rpcClient)
	return w.service, nil
}

func (w *remoteLogWriterGRPC) close() {
	if w.sharedConn {
		return
	}
	w.clientMutex.Lock()
	defer w.clientMutex.Unlock()

	if w.rpcClient != nil {
		utils.UncheckedErrorFunc(w.rpcClient.Close)
	}
}

// CreateNewGRPCClient creates a new grpc cloud configured to communicate with the robot service
// based on the cloud config given.
func CreateNewGRPCClient(ctx context.Context, cloudCfg *CloudConfig, logger Logger) (rpc.ClientConn, error) {
	grpcURL, err := url.Parse(cloudCfg.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)
	// Only add credentials when secret is set.
	if cloudCfg.Secret != "" {
		dialOpts = append(dialOpts, rpc.WithEntityCredentials(cloudCfg.ID,
			rpc.Credentials{
				Type:    "robot-secret",
				Payload: cloudCfg.Secret,
			},
		))
	}

	if grpcURL.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	return rpc.DialDirectGRPC(ctx, grpcURL.Host, logger, dialOpts...)
}

// A NetAppender must implement a zapcore such that it gets copied when downconverting on
// `Logger.AsZap`. The methods below are only invoked when passed as a zap.SugaredLogger to external
// libraries.
var _ zapcore.Core = &NetAppender{}

// Check checks if the entry should be logged. If so, add it to the CheckedEntry list of cores.
func (nl *NetAppender) Check(entry zapcore.Entry, checkedEntry *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if GlobalLogLevel.Enabled(entry.Level) {
		return checkedEntry.AddCore(entry, nl)
	}
	return checkedEntry
}

// Enabled returns if the NetAppender serving as a `zapcore.Core` should log.
func (nl *NetAppender) Enabled(level zapcore.Level) bool {
	return GlobalLogLevel.Enabled(level)
}

// With creates a zapcore.Core that will log like a `NetAppender` but with extra fields attached.
func (nl *NetAppender) With(f []zapcore.Field) zapcore.Core {
	return &wrappedLogger{nl, f}
}

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
