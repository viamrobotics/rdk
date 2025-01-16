// Package server implements the entry point for running a robot web server.
package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"slices"
	"sync"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
	rutils "go.viam.com/rdk/utils"
)

var viamDotDir = filepath.Join(rutils.PlatformHomeDir(), ".viam")

// Arguments for the command.
type Arguments struct {
	AllowInsecureCreds         bool   `flag:"allow-insecure-creds,usage=allow connections to send credentials over plaintext"`
	ConfigFile                 string `flag:"config,usage=robot config file"`
	CPUProfile                 string `flag:"cpuprofile,usage=write cpu profile to file"`
	Debug                      bool   `flag:"debug"`
	SharedDir                  string `flag:"shareddir,usage=web resource directory"`
	Version                    bool   `flag:"version,usage=print version"`
	WebProfile                 bool   `flag:"webprofile,usage=include profiler in http server"`
	WebRTC                     bool   `flag:"webrtc,default=true,usage=force webrtc connections instead of direct"`
	RevealSensitiveConfigDiffs bool   `flag:"reveal-sensitive-config-diffs,usage=show config diffs"`
	UntrustedEnv               bool   `flag:"untrusted-env,usage=disable processes and shell from running in a untrusted environment"`
	OutputTelemetry            bool   `flag:"output-telemetry,usage=print out telemetry data (metrics and spans)"`
	DisableMulticastDNS        bool   `flag:"disable-mdns,usage=disable server discovery through multicast DNS"`
	DumpResourcesPath          string `flag:"dump-resources,usage=dump all resource registrations as json to the provided file path"`
	EnableFTDC                 bool   `flag:"ftdc,default=true,usage=enable fulltime data capture for diagnostics"`
	OutputLogFile              string `flag:"log-file,usage=write logs to a file with log rotation"`
}

type robotServer struct {
	args     Arguments
	logger   logging.Logger
	registry *logging.Registry
}

func logViamEnvVariables(logger logging.Logger) {
	var viamEnvVariables []interface{}
	if value, exists := os.LookupEnv("VIAM_MODULE_ROOT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_MODULE_ROOT", value)
	}
	if value, exists := os.LookupEnv("VIAM_RESOURCE_CONFIGURATION_TIMEOUT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_RESOURCE_CONFIGURATION_TIMEOUT", value)
	}
	if value, exists := os.LookupEnv("VIAM_MODULE_STARTUP_TIMEOUT"); exists {
		viamEnvVariables = append(viamEnvVariables, "VIAM_MODULE_STARTUP_TIMEOUT", value)
	}
	if value, exists := os.LookupEnv("CWD"); exists {
		viamEnvVariables = append(viamEnvVariables, "CWD", value)
	}
	if rutils.PlatformHomeDir() != "" {
		viamEnvVariables = append(viamEnvVariables, "HOME", rutils.PlatformHomeDir())
	}
	if len(viamEnvVariables) != 0 {
		logger.Infow("Starting viam-server with following environment variables", viamEnvVariables...)
	}
}

func logVersion(logger logging.Logger) {
	var versionFields []interface{}
	if config.Version != "" {
		versionFields = append(versionFields, "version", config.Version)
	}
	if config.GitRevision != "" {
		versionFields = append(versionFields, "git_rev", config.GitRevision)
	}
	if len(versionFields) != 0 {
		logger.Infow("Viam RDK", versionFields...)
	} else {
		logger.Info("Viam RDK built from source; version unknown")
	}
}

func logStartupInfo(logger logging.Logger) {
	logVersion(logger)
	logViamEnvVariables(logger)
}

// RunServer is an entry point to starting the web server that can be called by main in a code
// sample or otherwise be used to initialize the server.
func RunServer(ctx context.Context, args []string, _ logging.Logger) (err error) {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	ctx, err = rutils.WithTrustedEnvironment(ctx, !argsParsed.UntrustedEnv)
	if err != nil {
		return err
	}

	if argsParsed.DumpResourcesPath != "" {
		return dumpResourceRegistrations(argsParsed.DumpResourcesPath)
	}

	logger, registry := logging.NewBlankLoggerWithRegistry("rdk")
	// Dan: We changed from a constructor that defaulted to INFO to `NewBlankLoggerWithRegistry`
	// which defaults to DEBUG. We pessimistically set the level to INFO to ensure parity. Though I
	// expect `InitLoggingSettings` will always put the logger into the right state without any
	// observable side-effects.
	logger.SetLevel(logging.INFO)
	if argsParsed.OutputLogFile != "" {
		logWriter, closer := logging.NewFileAppender(argsParsed.OutputLogFile)
		defer func() {
			utils.UncheckedError(closer.Close())
		}()
		logger.AddAppender(logWriter)
	} else {
		logger.AddAppender(logging.NewStdoutAppender())
	}

	logging.RegisterEventLogger(logger)
	logging.ReplaceGlobal(logger)
	config.InitLoggingSettings(logger, argsParsed.Debug)

	if argsParsed.Version {
		// log startup info here and return if version flag.
		logStartupInfo(logger)
		return
	}

	// log startup info locally if server fails and exits while attempting to start up
	var startupInfoLogged bool
	defer func() {
		if !startupInfoLogged {
			logger.CInfo(ctx, "error starting viam-server, logging version and exiting")
			logStartupInfo(logger)
		}
	}()

	if argsParsed.ConfigFile == "" {
		logger.Error("please specify a config file through the -config parameter.")
		return
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

	// Read the config from disk and use it to initialize the remote logger.
	initialReadCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	cfgFromDisk, err := config.ReadLocalConfig(initialReadCtx, argsParsed.ConfigFile, logger.Sublogger("config"))
	if err != nil {
		cancel()
		return err
	}
	cancel()

	if argsParsed.OutputTelemetry {
		exporter := perf.NewDevelopmentExporter()
		if err := exporter.Start(); err != nil {
			return err
		}
		defer exporter.Stop()
	}

	// Start remote logging with config from disk.
	// This is to ensure we make our best effort to write logs for failures loading the remote config.
	if cfgFromDisk.Cloud != nil && (cfgFromDisk.Cloud.LogPath != "" || cfgFromDisk.Cloud.AppAddress != "") {
		netAppender, err := logging.NewNetAppender(
			&logging.CloudConfig{
				AppAddress: cfgFromDisk.Cloud.AppAddress,
				ID:         cfgFromDisk.Cloud.ID,
				Secret:     cfgFromDisk.Cloud.Secret,
			},
			nil, false, logger.Sublogger("networking").Sublogger("netlogger"),
		)
		if err != nil {
			return err
		}
		defer netAppender.Close()

		registry.AddAppenderToAll(netAppender)
	}
	// log startup info after netlogger is initialized so it's captured in cloud machine logs.
	logStartupInfo(logger)
	startupInfoLogged = true

	server := robotServer{
		logger:   logger,
		args:     argsParsed,
		registry: registry,
	}

	// Run the server with remote logging enabled.
	err = server.runServer(ctx)
	if err != nil {
		logger.Error("Fatal error running server, exiting now: ", err)
	}

	return err
}

// runServer is an entry point to starting the web server after the local config is read. Once the local config
// is read the logger may be initialized to remote log. This ensure we capture errors starting up the server and report to the cloud.
func (s *robotServer) runServer(ctx context.Context) error {
	initialReadCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	cfg, err := config.Read(initialReadCtx, s.args.ConfigFile, s.logger)
	if err != nil {
		cancel()
		return err
	}
	cancel()
	config.UpdateFileConfigDebug(cfg.Debug)

	err = s.serveWeb(ctx, cfg)
	if err != nil {
		s.logger.Errorw("error serving web", "error", err)
	}

	return err
}

func (s *robotServer) createWebOptions(cfg *config.Config) (weboptions.Options, error) {
	options, err := weboptions.FromConfig(cfg)
	if err != nil {
		return weboptions.Options{}, err
	}
	options.Pprof = s.args.WebProfile || cfg.EnableWebProfile
	options.SharedDir = s.args.SharedDir
	options.Debug = s.args.Debug || cfg.Debug
	options.PreferWebRTC = s.args.WebRTC
	options.DisableMulticastDNS = s.args.DisableMulticastDNS
	if cfg.Cloud != nil && s.args.AllowInsecureCreds {
		options.SignalingDialOpts = append(options.SignalingDialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}

	if len(options.Auth.Handlers) == 0 {
		host, _, err := net.SplitHostPort(cfg.Network.BindAddress)
		if err != nil {
			return weboptions.Options{}, err
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			s.logger.Warn("binding to all interfaces without authentication")
		}
	}
	return options, nil
}

// A wrapper around actual config processing that also applies options from the
// robot server.
func (s *robotServer) processConfig(in *config.Config) (*config.Config, error) {
	out, err := config.ProcessConfig(in)
	if err != nil {
		return nil, err
	}
	out.Debug = s.args.Debug || in.Debug
	out.EnableWebProfile = s.args.WebProfile || in.EnableWebProfile
	out.FromCommand = true
	out.AllowInsecureCreds = s.args.AllowInsecureCreds
	out.UntrustedEnv = s.args.UntrustedEnv

	// Use ~/.viam/packages for package path if one was not specified.
	if in.PackagePath == "" {
		out.PackagePath = path.Join(viamDotDir, "packages")
	}

	return out, nil
}

// A function to be started as a goroutine that watches for changes, either
// from disk or from cloud, to the robot's config. Starts comparisons based on
// `currCfg`. Reconfigures the robot when config changes are received from the
// watcher.
func (s *robotServer) configWatcher(ctx context.Context, currCfg *config.Config, r robot.LocalRobot,
	watcher config.Watcher,
) {
	// Reconfigure robot to have passed-in config before listening for any config
	// changes.
	r.Reconfigure(ctx, currCfg)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		select {
		case <-ctx.Done():
			return
		case cfg := <-watcher.Config():
			processedConfig, err := s.processConfig(cfg)
			if err != nil {
				s.logger.Errorw("reconfiguration aborted: error processing config", "error", err)
				continue
			}

			// flag to restart web service if necessary
			diff, err := config.DiffConfigs(*currCfg, *processedConfig, s.args.RevealSensitiveConfigDiffs)
			if err != nil {
				s.logger.Errorw("reconfiguration aborted: error diffing config", "error", err)
				continue
			}
			var options weboptions.Options

			if !diff.NetworkEqual {
				// TODO(RSDK-2694): use internal web service reconfiguration instead
				r.StopWeb()
				options, err = s.createWebOptions(processedConfig)
				if err != nil {
					s.logger.Errorw("reconfiguration aborted: error creating weboptions", "error", err)
					continue
				}
			}

			// Update logger registry if log patterns may have changed.
			//
			// This functionality is tested in `TestLogPropagation` in `local_robot_test.go`.
			if !diff.LogEqual {
				s.logger.Debug("Detected potential changes to log patterns; updating logger levels")
				config.UpdateLoggerRegistryFromConfig(s.registry, processedConfig, s.logger)
			}

			r.Reconfigure(ctx, processedConfig)

			if !diff.NetworkEqual {
				if err := r.StartWeb(ctx, options); err != nil {
					s.logger.Errorw("reconfiguration failed: error starting web service while reconfiguring", "error", err)
				}
			}
			currCfg = processedConfig
		}
	}
}

func (s *robotServer) serveWeb(ctx context.Context, cfg *config.Config) (err error) {
	ctx, cancel := context.WithCancel(ctx)

	hungShutdownDeadline := 90 * time.Second
	slowWatcher, slowWatcherCancel := utils.SlowGoroutineWatcherAfterContext(
		ctx, hungShutdownDeadline, "server is taking a while to shutdown", s.logger)

	doneServing := make(chan struct{})

	forceShutdown := make(chan struct{})
	defer func() { <-forceShutdown }()

	var (
		theRobot                  robot.LocalRobot
		theRobotLock              sync.Mutex
		cloudRestartCheckerActive chan struct{}
	)
	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		if cloudRestartCheckerActive != nil {
			<-cloudRestartCheckerActive
		}
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	defer cancel()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)

	utils.PanicCapturingGo(func() {
		defer close(forceShutdown)

		<-ctx.Done()

		slowTicker := time.NewTicker(10 * time.Second)
		defer slowTicker.Stop()

		checkDone := func() bool {
			select {
			case <-slowWatcher:
				select {
				// the successful shutdown case has us close(doneServing), followed by slowWatcherCancel,
				// meaning both may be selected so we check to see if doneServing was also closed. If the
				// deadline truly elapses, there's a chance we shutdown cleanly at the exact same time which may
				// result in not catching this case.
				case <-doneServing:
					return true
				default:
					theRobotLock.Lock()
					robot := theRobot
					theRobotLock.Unlock()
					if robot != nil {
						robot.Kill()
					}
					s.logger.Fatalw("server failed to cleanly shutdown after deadline", "deadline", hungShutdownDeadline)
					return true
				}
			case <-doneServing:
				return true
			default:
				return false
			}
		}

		for {
			select {
			case <-slowWatcher:
				if checkDone() {
					return
				}
			case <-doneServing:
				return
			case <-slowTicker.C:
				if checkDone() {
					return
				}
				s.logger.Warn("waiting for clean shutdown")
			}
		}
	})

	defer func() {
		close(doneServing)
		slowWatcherCancel()
		<-slowWatcher
	}()

	fullProcessedConfig, err := s.processConfig(cfg)
	if err != nil {
		return err
	}

	// Update logger registry as soon as we have fully processed config. Further
	// updates to the registry will be handled by the config watcher goroutine.
	//
	// This functionality is tested in `TestLogPropagation` in `local_robot_test.go`.
	config.UpdateLoggerRegistryFromConfig(s.registry, fullProcessedConfig, s.logger)

	if fullProcessedConfig.Cloud != nil {
		cloudRestartCheckerActive = make(chan struct{})
		utils.PanicCapturingGo(func() {
			defer close(cloudRestartCheckerActive)
			restartCheck, err := newRestartChecker(ctx, cfg.Cloud, s.logger)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				s.logger.Errorw("error creating restart checker", "error", err)
				panic(fmt.Sprintf("error creating restart checker: %v", err))
			}
			defer restartCheck.close()
			restartInterval := defaultNeedsRestartCheckInterval

			for {
				if !utils.SelectContextOrWait(ctx, restartInterval) {
					return
				}

				mustRestart, newRestartInterval, err := restartCheck.needsRestart(ctx)
				if err != nil {
					s.logger.Infow("failed to check restart", "error", err)
					continue
				}

				restartInterval = newRestartInterval

				if mustRestart {
					logStackTraceAndCancel(cancel, s.logger)
				}
			}
		})
	}

	robotOptions := createRobotOptions()
	if s.args.RevealSensitiveConfigDiffs {
		robotOptions = append(robotOptions, robotimpl.WithRevealSensitiveConfigDiffs())
	}

	shutdownCallbackOpt := robotimpl.WithShutdownCallback(func() {
		logStackTraceAndCancel(cancel, s.logger)
	})
	robotOptions = append(robotOptions, shutdownCallbackOpt)

	if s.args.EnableFTDC {
		robotOptions = append(robotOptions, robotimpl.WithFTDC())
	}

	// Create `minimalProcessedConfig`, a copy of `fullProcessedConfig`. Remove
	// all components, services, remotes, modules, and processes from
	// `minimalProcessedConfig`. Create new robot with `minimalProcessedConfig`
	// and immediately start web service. We need the machine to be reachable
	// through the web service ASAP, even if some resources take a long time to
	// initially configure.
	minimalProcessedConfig := *fullProcessedConfig
	minimalProcessedConfig.Components = nil
	minimalProcessedConfig.Services = nil
	minimalProcessedConfig.Remotes = nil
	minimalProcessedConfig.Modules = nil
	minimalProcessedConfig.Processes = nil

	// Mark minimalProcessedConfig as an initial config, so robot reports a
	// state of initializing until reconfigured with full config.
	minimalProcessedConfig.Initial = true

	myRobot, err := robotimpl.New(ctx, &minimalProcessedConfig, s.logger, robotOptions...)
	if err != nil {
		cancel()
		return err
	}
	theRobotLock.Lock()
	theRobot = myRobot
	theRobotLock.Unlock()
	defer func() {
		err = multierr.Combine(err, theRobot.Close(context.Background()))
	}()

	// watch for and deliver changes to the robot
	watcher, err := config.NewWatcher(ctx, cfg, s.logger.Sublogger("config"))
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		err = multierr.Combine(err, watcher.Close())
	}()

	onWatchDone := make(chan struct{})
	go func() {
		defer close(onWatchDone)

		// Use `fullProcessedConfig` as the initial config for the config watcher
		// goroutine, as we want incoming config changes to be compared to the full
		// config.
		s.configWatcher(ctx, fullProcessedConfig, theRobot, watcher)
	}()
	// At end of this function, cancel context and wait for watcher goroutine
	// to complete.
	defer func() {
		cancel()
		<-onWatchDone
	}()

	// Create initial web options with `minimalProcessedConfig`.
	options, err := s.createWebOptions(&minimalProcessedConfig)
	if err != nil {
		return err
	}
	return web.RunWeb(ctx, theRobot, options, s.logger)
}

// dumpResourceRegistrations prints all builtin resource registrations as a json array
// to the provided file. If you edit this function, ensure that etc/system_manifest/main.go is
// updated correspondingly.
func dumpResourceRegistrations(outputPath string) error {
	type resourceRegistration struct {
		API   string `json:"api"`
		Model string `json:"model"`
		// AttributeSchema is a serialization of the Go resource "Config" structures that components and services Reconfigure with.
		// Notably this includes the JSON tags that are used to parse these resource configs from the robot's JSON config.
		AttributeSchema *jsonschema.Schema `json:"attribute_schema,omitempty"`
	}

	// create the array of all resource registrations
	resources := make([]resourceRegistration, 0, len(resource.RegisteredResources()))
	for apimodel, reg := range resource.RegisteredResources() {
		var attributeSchema *jsonschema.Schema
		reflectType := reg.ConfigReflectType()
		if reflectType != nil {
			attributeSchema = jsonschema.ReflectFromType(reflectType)
		}
		resources = append(resources, resourceRegistration{
			API:             apimodel.API.String(),
			Model:           apimodel.Model.String(),
			AttributeSchema: attributeSchema,
		})
	}

	// sort the list alphabetically by API+Model
	slices.SortFunc(resources, func(a, b resourceRegistration) int {
		if a.API != b.API {
			return cmp.Compare(a.API, b.API)
		}
		return cmp.Compare(a.Model, b.Model)
	})

	// marshall and print the registrations to the provided file
	jsonResult, err := json.MarshalIndent(resources, "", "\t")
	if err != nil {
		return errors.Wrap(err, "unable to marshall resources")
	}

	if err := os.WriteFile(outputPath, jsonResult, 0o600); err != nil {
		return errors.Wrap(err, "unable to write resulting object to stdout")
	}
	return nil
}

func logStackTraceAndCancel(cancel context.CancelFunc, logger logging.Logger) {
	bufSize := 1 << 20
	traces := make([]byte, bufSize)
	traceSize := runtime.Stack(traces, true)
	message := "backtrace at robot shutdown"
	if traceSize == bufSize {
		message = fmt.Sprintf("%s (warning: backtrace truncated to %v bytes)", message, bufSize)
	}
	logger.Infof("%s, %s", message, traces[:traceSize])
	cancel()
}
