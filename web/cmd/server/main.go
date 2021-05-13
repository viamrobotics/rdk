// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

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

	"go.viam.com/core/config"
	"go.viam.com/core/rlog"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/rpc"
	"go.viam.com/core/utils"
	"go.viam.com/core/web"

	// These are the robot pieces we want by default
	_ "go.viam.com/core/base/impl"
	_ "go.viam.com/core/board/detector"
	_ "go.viam.com/core/rimage/imagesource"
	_ "go.viam.com/core/robots/eva" // for eva
	_ "go.viam.com/core/robots/hellorobot"
	_ "go.viam.com/core/robots/robotiq"         // for a gripper
	_ "go.viam.com/core/robots/softrobotics"    // for a gripper
	_ "go.viam.com/core/robots/universalrobots" // for an arm
	_ "go.viam.com/core/robots/varm"            // for an arm
	_ "go.viam.com/core/robots/vgripper"        // for a gripper
	_ "go.viam.com/core/robots/vx300s"          // for arm and gripper
	_ "go.viam.com/core/robots/wx250s"          // for arm and gripper

	"github.com/edaniels/golog"
	"github.com/erh/egoutil"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

var logger = golog.NewDevelopmentLogger("robot_server")

func newNetLogger(config *config.Cloud) (*netLogger, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	nl := &netLogger{hostname: hostname, cfg: config, cancel: cancel}
	nl.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		nl.backgroundWorker(cancelCtx)
	}, nl.activeBackgroundWorkers.Done)
	return nl, nil
}

type netLogger struct {
	hostname string
	cfg      *config.Cloud

	toLogMutex sync.Mutex
	toLog      []interface{}

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
	panic(1)
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
		return fmt.Errorf("error creating log request %w", err)
	}
	r.Header.Set("Secret", nl.cfg.Secret)

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer utils.UncheckedError(resp.Body.Close())
	return nil
}

func (nl *netLogger) backgroundWorker(ctx context.Context) {
	for {
		if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
			return
		}
		err := nl.Sync()
		if err != nil {
			// fall back to regular logging
			rlog.Logger.Infof("error logging to network: %s", err)
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
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
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

	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)
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

	options := web.NewOptions()
	options.AutoTile = !argsParsed.NoAutoTile
	options.Pprof = argsParsed.WebProfile
	options.Port = int(argsParsed.Port)
	options.SharedDir = argsParsed.SharedDir

	err = web.RunWeb(ctx, myRobot, options, logger)
	if err != nil {
		logger.Errorw("error running web", "error", err)
	}
	<-onWatchDone
	return err
}
