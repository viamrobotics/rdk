package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	"go.uber.org/multierr"

	"go.viam.com/utils"
	"go.viam.com/utils/rpc/dialer"

	"go.viam.com/core/config"
	"go.viam.com/core/metadata/service"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"

	// These are the robot pieces we want by default
	_ "go.viam.com/core/base/impl"
	_ "go.viam.com/core/board/arduino"
	_ "go.viam.com/core/board/detector"
	_ "go.viam.com/core/board/jetson"
	_ "go.viam.com/core/input/gamepad" // xbox controller and similar
	_ "go.viam.com/core/motor/gpio"
	_ "go.viam.com/core/motor/gpiostepper"
	_ "go.viam.com/core/rimage/imagesource"
	_ "go.viam.com/core/robots/eva"                     // for eva
	_ "go.viam.com/core/robots/gopro"                   // for a camera
	_ "go.viam.com/core/robots/robotiq"                 // for a gripper
	_ "go.viam.com/core/robots/softrobotics"            // for a gripper
	_ "go.viam.com/core/robots/universalrobots"         // for an arm
	_ "go.viam.com/core/robots/varm"                    // for an arm
	_ "go.viam.com/core/robots/vforcematrixtraditional" // for a traditional force matrix
	_ "go.viam.com/core/robots/vforcematrixwithmux"     // for a force matrix built using a mux
	_ "go.viam.com/core/robots/vgripper"                // for a gripper
	_ "go.viam.com/core/robots/vx300s"                  // for arm and gripper
	_ "go.viam.com/core/robots/wx250s"                  // for arm and gripper
	_ "go.viam.com/core/robots/xarm"                    // for an arm

	"github.com/edaniels/golog"
	"github.com/erh/egoutil"
	"github.com/go-errors/errors"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type wrappedLogger struct {
	base  zapcore.Core
	extra []zapcore.Field
}

func (l *wrappedLogger) Close() error {
	return nil
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
	new := []zapcore.Field{}
	new = append(new, l.extra...)
	new = append(new, f...)
	return l.base.Write(e, new)
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
	utils.ManagedGo(func() {
		nl.backgroundWorker()
	}, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

type netLogger struct {
	hostname string
	cfg      *config.Cloud

	toLogMutex sync.Mutex
	toLog      []interface{}

	cancelCtx               context.Context
	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
}

func (nl *netLogger) Close() error {
	nl.cancel()
	nl.activeBackgroundWorkers.Wait()
	return nil
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

	r, err := http.NewRequest("POST", nl.cfg.LogPath, bytes.NewReader(js))
	if err != nil {
		return errors.Errorf("error creating log request %w", err)
	}
	r.Header.Set("Secret", nl.cfg.Secret)
	r = r.WithContext(nl.cancelCtx)

	var client http.Client
	defer client.CloseIdleConnections()
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)
	return nil
}

func (nl *netLogger) backgroundWorker() {
	normalInterval := 100 * time.Millisecond
	abnormalInterval := 5 * time.Second
	interval := normalInterval
	for {
		if !utils.SelectContextOrWait(nl.cancelCtx, interval) {
			return
		}
		err := nl.Sync()
		if err != nil {
			interval = abnormalInterval
			// fall back to regular logging
			rlog.Logger.Infof("error logging to network: %s", err)
		} else {
			interval = normalInterval
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

func addCloudLogger(logger golog.Logger, cfg *config.Cloud) (golog.Logger, func() error, error) {
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
	ConfigFile string            `flag:"0,required,usage=robot config file"`
	NoAutoTile bool              `flag:"noAutoTile,usage=disable auto tiling"`
	CPUProfile string            `flag:"cpuprofile,usage=write cpu profile to file"`
	WebProfile bool              `flag:"webprofile,usage=include profiler in http server"`
	LogURL     string            `flag:"logurl,usage=url to log messages to"`
	SharedDir  string            `flag:"shareddir,usage=web resource directory"`
	Port       utils.NetPortFlag `flag:"port,usage=port to listen on"`
	Debug      bool              `flag:"debug"`
	LocalCloud bool              `flag:"local-cloud"`
	WebRTC     bool              `flag:"webrtc,usage=force webrtc connections instead of direct"`
}

// RunServer is an entry point to starting the web server that can be called by main in a code sample or otherwise be used to initialize the server.
func RunServer(ctx context.Context, args []string, logger golog.Logger) (err error) {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}
	if argsParsed.Port == 0 {
		argsParsed.Port = 8080
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

	exp := egoutil.NewNiceLoggingSpanExporter()
	trace.RegisterExporter(exp)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	cfg, err := config.Read(argsParsed.ConfigFile)
	if err != nil {
		return err
	}

	if cfg.Cloud != nil && cfg.Cloud.LogPath != "" {
		var cleanup func() error
		logger, cleanup, err = addCloudLogger(logger, cfg.Cloud)
		if err != nil {
			return err
		}
		defer func() {
			err = multierr.Combine(err, cleanup())
		}()
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
	rpcDialer := dialer.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = dialer.ContextWithDialer(ctx, rpcDialer)

	metadataSvc, err := service.New()
	if err != nil {
		return err
	}
	ctx = service.ContextWithService(ctx, metadataSvc)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}

	// watch for and deliver changes to the robot
	watcher, err := config.NewWatcher(cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, watcher.Close())
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
				if err := myRobot.Reconfigure(ctx, config); err != nil {
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

	options := web.NewOptions()
	options.AutoTile = !argsParsed.NoAutoTile
	options.Pprof = argsParsed.WebProfile
	options.Port = int(argsParsed.Port)
	options.SharedDir = argsParsed.SharedDir
	options.Debug = argsParsed.Debug
	options.WebRTC = argsParsed.WebRTC
	if cfg.Cloud != nil {
		options.Name = cfg.Cloud.Self
		options.SignalingAddress = cfg.Cloud.SignalingAddress
		if argsParsed.LocalCloud {
			options.Insecure = true
		}
	} else {
		options.Insecure = true
	}

	err = RunWeb(ctx, myRobot, options, logger)
	if err != nil {
		cancel()
		return fmt.Errorf("error running web: %w", err)
	}
	return err
}
