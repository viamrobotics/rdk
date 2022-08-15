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
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/rlog"
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

	cancelCtx, cancel := context.WithCancel(context.Background())
	nl := &netLogger{
		hostname:  hostname,
		cfg:       config,
		cancelCtx: cancelCtx,
		cancel:    cancel,
	}
	nl.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(nl.backgroundWorker, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

type netLogger struct {
	hostname   string
	cfg        *config.Cloud
	httpClient http.Client

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
	nl.httpClient.CloseIdleConnections()
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

	if len(nl.toLog) > 20000 {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[1:]
	}
	nl.toLog = append(nl.toLog, x)
}

func (nl *netLogger) writeToServer(log *apppb.LogEntry) error {
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
		"id":     nl.cfg.ID,
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
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, nl.cfg.LogPath, bytes.NewReader(js))
	if err != nil {
		return errors.Wrap(err, "error creating log request")
	}
	r.Header.Set("Secret", nl.cfg.Secret)

	resp, err := nl.httpClient.Do(r)
	if err != nil {
		return err
	}
	defer func() {
		utils.UncheckedError(resp.Body.Close())
	}()
	return nil
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
	// TODO(erh): batch writes

	for {
		x := func() *apppb.LogEntry {
			nl.toLogMutex.Lock()
			defer nl.toLogMutex.Unlock()

			if len(nl.toLog) == 0 {
				return nil
			}

			x := nl.toLog[0]
			nl.toLog = nl.toLog[1:]

			return x
		}()

		if x == nil {
			return nil
		}

		err := nl.writeToServer(x)
		if err != nil {
			nl.addToQueue(x) // we'll try again later
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
