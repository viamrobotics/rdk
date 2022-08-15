// Package server implements the entry point for running a robot web server.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	apppb "go.viam.com/api/proto/viam/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/rlog"
)

const (
	maxQueueSize   = 20000
	writeBatchSize = 100
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

func newNetLogger(config *config.Cloud) (*netLogger, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	var logWriter remoteLogWriter
	if config.AppAddress == "" {
		logWriter = &remoteLogWriterHTTP{
			cfg:    config,
			client: http.Client{},
		}
	} else {
		logWriter = &remoteLogWriterGRPC{
			logger: rlog.Logger,
			cfg:    config,
		}
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	nl := &netLogger{
		hostname:     hostname,
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		remoteWriter: logWriter,
	}
	nl.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(nl.backgroundWorker, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

type netLogger struct {
	hostname     string
	remoteWriter remoteLogWriter

	toLogMutex sync.Mutex
	toLog      []*apppb.LogEntry

	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
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

func (nl *netLogger) Enabled(zapcore.Level) bool {
	return true
}

func (nl *netLogger) With(f []zapcore.Field) zapcore.Core {
	return &wrappedLogger{nl, f}
}

func (nl *netLogger) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(e, nl)
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

	if len(nl.toLog) > maxQueueSize {
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

	if len(nl.toLog) > maxQueueSize {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[len(x):]
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
			// fall back to regular logging
			rlog.Logger.Infof("error logging to network: %s", err)
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

			x := nl.toLog[0:batchSize]
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

func addCloudLogger(logger golog.Logger, cfg *config.Cloud) (golog.Logger, func(), error) {
	nl, err := newNetLogger(cfg)
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

type remoteLogWriterHTTP struct {
	cfg    *config.Cloud
	client http.Client
}

func (w *remoteLogWriterHTTP) write(logs []*apppb.LogEntry) error {
	for _, log := range logs {
		err := w.writeToServer(log)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *remoteLogWriterHTTP) writeToServer(log *apppb.LogEntry) error {
	level, err := zapcore.ParseLevel(log.Level)
	if err != nil {
		return errors.Wrap(err, "error creating log request")
	}

	e := zapcore.Entry{
		Level:      level,
		LoggerName: log.LoggerName,
		Message:    log.Message,
		Stack:      log.Stack,
		Time:       log.Time.AsTime(),
	}

	x := map[string]interface{}{
		"id":     w.cfg.ID,
		"host":   log.Host,
		"log":    e,
		"fields": log.Fields,
	}

	js, err := json.Marshal(x)
	if err != nil {
		return err
	}

	// we specifically don't use a parented cancellable context here so we can make sure we finish writing but
	// we will only give it up to 5 seconds to do so in case we are trying to shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, w.cfg.LogPath, bytes.NewReader(js))
	if err != nil {
		return errors.Wrap(err, "error creating log request")
	}
	r.Header.Set("Secret", w.cfg.Secret)

	resp, err := w.client.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(resp.Body.Close())
	}()
	return nil
}

func (w *remoteLogWriterHTTP) close() {
	w.client.CloseIdleConnections()
}

type remoteLogWriterGRPC struct {
	cfg         *config.Cloud
	service     apppb.RobotServiceClient
	rpcClient   rpc.ClientConn
	clientMutex sync.Mutex
	logger      golog.Logger
}

func (w *remoteLogWriterGRPC) write(logs []*apppb.LogEntry) error {
	client, err := w.getOrCreateClient()
	if err != nil {
		return err
	}

	_, err = client.Log(context.Background(), &apppb.LogRequest{Id: w.cfg.ID, Logs: logs})
	if err != nil {
		return err
	}

	return nil
}

func (w *remoteLogWriterGRPC) getOrCreateClient() (apppb.RobotServiceClient, error) {
	if w.service == nil {
		w.clientMutex.Lock()
		defer w.clientMutex.Unlock()

		client, err := config.CreateNewGRPCClient(context.Background(), w.cfg, w.logger)
		if err != nil {
			return nil, err
		}

		w.rpcClient = client
		w.service = apppb.NewRobotServiceClient(w.rpcClient)
	}
	return w.service, nil
}

func (w *remoteLogWriterGRPC) close() {
	if w.rpcClient != nil {
		utils.UncheckedErrorFunc(w.rpcClient.Close)
	}
}
