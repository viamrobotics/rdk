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
	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/robot"
	"go.viam.com/robotcore/robot/web"
	"go.viam.com/robotcore/rpc"
	"go.viam.com/robotcore/utils"

	// These are the robot pieces we want by default
	_ "go.viam.com/robotcore/board/detector"
	_ "go.viam.com/robotcore/rimage/imagesource"
	_ "go.viam.com/robotcore/robots/eva" // for eva
	_ "go.viam.com/robotcore/robots/hellorobot"
	_ "go.viam.com/robotcore/robots/robotiq"         // for a gripper
	_ "go.viam.com/robotcore/robots/softrobotics"    // for a gripper
	_ "go.viam.com/robotcore/robots/universalrobots" // for an arm
	_ "go.viam.com/robotcore/robots/vgripper"        // for a gripper
	_ "go.viam.com/robotcore/robots/vx300s"          // for arm and gripper
	_ "go.viam.com/robotcore/robots/wx250s"          // for arm and gripper

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

func NewNetLogger(config api.CloudConfig) (zapcore.Core, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	nl := &netLogger{hostname: hostname, cfg: config}
	go nl.backgroundThread()
	return nl, nil
}

type netLogger struct {
	hostname string
	cfg      api.CloudConfig

	toLogMutex sync.Mutex
	toLog      []interface{}
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
	defer resp.Body.Close()
	return nil
}

func (nl *netLogger) backgroundThread() {
	for {
		time.Sleep(100 * time.Millisecond)
		err := nl.Sync()
		if err != nil {
			// fall back to regular logging
			golog.Global.Infof("error logging to network: %s", err)
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

func addCloudLogger(logger golog.Logger, cfg api.CloudConfig) (golog.Logger, error) {
	nl, err := NewNetLogger(cfg)
	if err != nil {
		return nil, err
	}
	l := logger.Desugar()
	l = l.WithOptions(zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(c, nl)
	}))
	return l.Sugar(), nil
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

	cfg, err := api.ReadConfig(argsParsed.ConfigFile)
	if err != nil {
		return err
	}

	if cfg.Cloud.LogPath != "" {
		logger, err = addCloudLogger(logger, cfg.Cloud)
		if err != nil {
			return err
		}
	}

	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)
	myRobot, err := robot.NewRobot(ctx, cfg, logger)
	if err != nil {
		return err
	}

	options := web.NewOptions()
	options.AutoTile = !argsParsed.NoAutoTile
	options.Pprof = argsParsed.WebProfile
	options.Port = int(argsParsed.Port)
	options.SharedDir = argsParsed.SharedDir

	return web.RunWeb(ctx, myRobot, options, logger)
}
