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
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultMaxQueueSize = 20000
	writeBatchSize      = 100
)

// CloudConfig contains the necessary inputs to send logs to the app backend over grpc.
type CloudConfig struct {
	AppAddress string
	ID         string
	Secret     string
}

// NewNetAppender creates a NetAppender to send log events to the app backend.
func NewNetAppender(config *CloudConfig) (*NetAppender, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	logWriter := &remoteLogWriterGRPC{
		cfg: config,
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	nl := &NetAppender{
		hostname:         hostname,
		cancelCtx:        cancelCtx,
		cancel:           cancel,
		remoteWriter:     logWriter,
		maxQueueSize:     defaultMaxQueueSize,
		loggerWithoutNet: NewLogger("netlogger"),
	}
	nl.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(nl.backgroundWorker, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

// NetAppender can send log events to the app backend.
type NetAppender struct {
	hostname     string
	remoteWriter *remoteLogWriterGRPC

	toLogMutex   sync.Mutex
	toLog        []*apppb.LogEntry
	maxQueueSize int

	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup

	// the netLogger causing a recursive loop.
	loggerWithoutNet Logger
}

func (nl *NetAppender) queueSize() int {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()
	return len(nl.toLog)
}

// Close the NetAppender. This makes a best effort at sending all logs before returning.
func (nl *NetAppender) Close() {
	// try for up to 10 seconds for log queue to clear before cancelling it
	for i := 0; i < 1000; i++ {
		// A batch can be popped from the queue for a sync by the background worker, and re-enqueued
		// due to an error. This check does not account for this case. It will cancel the background
		// worker once the last batch is in flight. Successful or not.
		if nl.queueSize() == 0 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}
	nl.cancel()
	nl.activeBackgroundWorkers.Wait()
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

func (nl *NetAppender) addToQueue(logEntry *apppb.LogEntry) {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(nl.toLog) >= nl.maxQueueSize {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[1:]
	}
	nl.toLog = append(nl.toLog, logEntry)
}

func (nl *NetAppender) addBatchToQueue(batch []*apppb.LogEntry) {
	if len(batch) == 0 {
		return
	}

	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(batch) > nl.maxQueueSize {
		batch = batch[len(batch)-nl.maxQueueSize:]
	}

	if len(nl.toLog)+len(batch) >= nl.maxQueueSize {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[len(nl.toLog)+len(batch)-nl.maxQueueSize:]
	}

	nl.toLog = append(nl.toLog, batch...)
}

func (nl *NetAppender) backgroundWorker() {
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

// Sync will flush the internal buffer of logs.
func (nl *NetAppender) Sync() error {
	for {
		batch := func() []*apppb.LogEntry {
			nl.toLogMutex.Lock()
			defer nl.toLogMutex.Unlock()

			if len(nl.toLog) == 0 {
				return nil
			}

			batchSize := writeBatchSize
			if len(nl.toLog) < writeBatchSize {
				batchSize = len(nl.toLog)
			}

			ret := nl.toLog[:batchSize]
			nl.toLog = nl.toLog[batchSize:]

			return ret
		}()

		if len(batch) == 0 {
			return nil
		}

		err := nl.remoteWriter.write(batch)
		if err != nil {
			// On an error, the failed batch gets put on the back of the queue. Logs can be sent out
			// of order. We depend on log front-ends to sort the results by time if they care about
			// order.
			nl.addBatchToQueue(batch)
			return err
		}
	}
}

type remoteLogWriterGRPC struct {
	cfg *CloudConfig

	// `service` and `rpcClient` are lazily initialized on first call to `write`. The `clientMutex`
	// serializes access to ensure that only one caller creates them.
	service     apppb.RobotServiceClient
	rpcClient   rpc.ClientConn
	clientMutex sync.Mutex
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

	client, err := CreateNewGRPCClient(ctx, w.cfg)
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

// CreateNewGRPCClient creates a new grpc cloud configured to communicate with the robot service based on the cloud config given.
func CreateNewGRPCClient(ctx context.Context, cloudCfg *CloudConfig) (rpc.ClientConn, error) {
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

	return rpc.DialDirectGRPC(ctx, grpcURL.Host, NewLogger("netlogger").AsZap(), dialOpts...)
}
