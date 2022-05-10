// Package server implements the entry point for running a robot web server.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rlog"
	robotimpl "go.viam.com/rdk/robot/impl"
	web "go.viam.com/rdk/robot/web"
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
	toLog      []interface{}

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

func (nl *netLogger) Write(e zapcore.Entry, f []zapcore.Field) error {
	// TODO(erh): should we put a _id uuid on here so we don't log twice?

	for idx, ff := range f {
		if ff.String == "" && ff.Interface != nil {
			ff.String = fmt.Sprintf("%v", ff.Interface)
			f[idx] = ff
		}
	}

	x := map[string]interface{}{
		"id":     nl.cfg.ID,
		"host":   nl.hostname,
		"log":    e,
		"fields": f,
	}

	nl.addToQueue(x)

	if e.Level == zapcore.FatalLevel || e.Level == zapcore.DPanicLevel || e.Level == zapcore.PanicLevel {
		// program is going to go away, let's try and sync all our messages before then
		return nl.Sync()
	}

	return nil
}

func (nl *netLogger) addToQueue(x interface{}) {
	nl.toLogMutex.Lock()
	defer nl.toLogMutex.Unlock()

	if len(nl.toLog) > 20000 {
		// TODO(erh): sample?
		nl.toLog = nl.toLog[1:]
	}
	nl.toLog = append(nl.toLog, x)
}

func (nl *netLogger) writeToServer(x interface{}) error {
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
		x := func() interface{} {
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

// Arguments for the command.
type Arguments struct {
	ConfigFile         string `flag:"0,required,usage=robot config file"`
	CPUProfile         string `flag:"cpuprofile,usage=write cpu profile to file"`
	WebProfile         bool   `flag:"webprofile,usage=include profiler in http server"`
	LogURL             string `flag:"logurl,usage=url to log messages to"`
	SharedDir          string `flag:"shareddir,usage=web resource directory"`
	Debug              bool   `flag:"debug"`
	WebRTC             bool   `flag:"webrtc,usage=force webrtc connections instead of direct"`
	AllowInsecureCreds bool   `flag:"allow-insecure-creds,usage=allow connections to send credentials over plaintext"`
}

// RunServer is an entry point to starting the web server that can be called by main in a code
// sample or otherwise be used to initialize the server.
func RunServer(ctx context.Context, args []string, logger golog.Logger) (err error) {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	if argsParsed.CPUProfile != "" {
		f, err := os.Create(argsParsed.CPUProfile)
		if err != nil {
			return err
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}

	exp := perf.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	initialReadCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	cfg, err := config.Read(initialReadCtx, argsParsed.ConfigFile, logger)
	if err != nil {
		cancel()
		return err
	}
	cancel()

	if cfg.Cloud != nil && cfg.Cloud.LogPath != "" {
		var closer func()
		logger, closer, err = addCloudLogger(logger, cfg.Cloud)
		if err != nil {
			return err
		}
		defer closer()
	}

	err = serveWeb(ctx, cfg, argsParsed, logger)
	if err != nil {
		logger.Errorw("error serving web", "error", err)
	}
	return err
}

func serveWeb(ctx context.Context, cfg *config.Config, argsParsed Arguments, logger golog.Logger) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)

	processConfig := func(in *config.Config) (*config.Config, error) {
		tlsCfg := config.NewTLSConfig(cfg)
		out, err := config.ProcessConfig(in, tlsCfg)
		if err != nil {
			return nil, err
		}
		out.Debug = argsParsed.Debug
		out.FromCommand = true
		out.AllowInsecureCreds = argsParsed.AllowInsecureCreds
		return out, nil
	}

	processedConfig, err := processConfig(cfg)
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.New(ctx, processedConfig, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, myRobot.Close(context.Background()))
	}()

	// watch for and deliver changes to the robot
	watcher, err := config.NewWatcher(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, utils.TryClose(ctx, watcher))
	}()
	onWatchDone := make(chan struct{})
	utils.ManagedGo(func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			select {
			case <-ctx.Done():
				return
			case config := <-watcher.Config():
				processedConfig, err := processConfig(config)
				if err != nil {
					logger.Errorw("error processing config", "error", err)
				}
				if err := myRobot.Reconfigure(ctx, processedConfig); err != nil {
					logger.Errorw("error reconfiguring robot", "error", err)
				}
			}
		}
	}, func() {
		close(onWatchDone)
	})
	defer func() {
		<-onWatchDone
	}()
	defer cancel()

	options, err := web.OptionsFromConfig(processedConfig)
	if err != nil {
		return err
	}
	options.Pprof = argsParsed.WebProfile
	options.SharedDir = argsParsed.SharedDir
	options.Debug = argsParsed.Debug
	options.WebRTC = argsParsed.WebRTC
	if cfg.Cloud != nil && argsParsed.AllowInsecureCreds {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}

	if len(options.Auth.Handlers) == 0 {
		host, _, err := net.SplitHostPort(cfg.Network.BindAddress)
		if err != nil {
			return err
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			logger.Warn("binding to all interfaces without authentication")
		}
	}

	return robotimpl.RunWeb(ctx, myRobot, options, logger)
}
