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
	"time"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/perf"
	"go.viam.com/utils/rpc"

	vlogging "go.viam.com/rdk/components/camera/videosource/logging"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
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
	Logging                    bool   `flag:"logging,default=false,usage=emit periodic resource status information to Viam's hidden folder"`
	SharedDir                  string `flag:"shareddir,usage=web resource directory"`
	Version                    bool   `flag:"version,usage=print version"`
	WebProfile                 bool   `flag:"webprofile,usage=include profiler in http server"`
	WebRTC                     bool   `flag:"webrtc,default=true,usage=force webrtc connections instead of direct"`
	RevealSensitiveConfigDiffs bool   `flag:"reveal-sensitive-config-diffs,usage=show config diffs"`
	UntrustedEnv               bool   `flag:"untrusted-env,usage=disable processes and shell from running in a untrusted environment"`
	OutputTelemetry            bool   `flag:"output-telemetry,usage=print out telemetry data (metrics and spans)"`
	DisableMulticastDNS        bool   `flag:"disable-mdns,usage=disable server discovery through multicast DNS"`
	DumpResourcesPath          string `flag:"dump-resources,usage=dump all resource registrations as json to the provided file path"`
}

type robotServer struct {
	args   Arguments
	logger logging.Logger
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

	// Replace logger with logger based on flags.
	logger := logging.NewLogger("")
	logging.ReplaceGlobal(logger)
	logger = logger.Sublogger("robot_server")
	config.InitLoggingSettings(logger, argsParsed.Debug)

	// Always log the version, return early if the '-version' flag was provided
	// fmt.Println would be better but fails linting. Good enough.
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
	if argsParsed.Version {
		return
	}

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

	if argsParsed.Logging {
		if err := vlogging.GLoggerCamComp.Start(ctx); err != nil {
			logger.Debug(err)
		}
	}

	// Read the config from disk and use it to initialize the remote logger.
	initialReadCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	cfgFromDisk, err := config.ReadLocalConfig(initialReadCtx, argsParsed.ConfigFile, logger)
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
			nil,
		)
		if err != nil {
			return err
		}
		defer netAppender.Close()

		logger.AddAppender(netAppender)
	}

	server := robotServer{
		logger: logger,
		args:   argsParsed,
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
	options.WebRTC = s.args.WebRTC
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

func (s *robotServer) serveWeb(ctx context.Context, cfg *config.Config) (err error) {
	ctx, cancel := context.WithCancel(ctx)

	hungShutdownDeadline := 90 * time.Second
	slowWatcher, slowWatcherCancel := utils.SlowGoroutineWatcherAfterContext(
		ctx, hungShutdownDeadline, "server is taking a while to shutdown", s.logger.AsZap())

	doneServing := make(chan struct{})

	forceShutdown := make(chan struct{})
	defer func() { <-forceShutdown }()

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

	var cloudRestartCheckerActive chan struct{}
	rpcDialer := rpc.NewCachedDialer()
	defer func() {
		if cloudRestartCheckerActive != nil {
			<-cloudRestartCheckerActive
		}
		err = multierr.Combine(err, rpcDialer.Close())
	}()
	defer cancel()
	ctx = rpc.ContextWithDialer(ctx, rpcDialer)

	processConfig := func(in *config.Config) (*config.Config, error) {
		tlsCfg := config.NewTLSConfig(cfg)
		out, err := config.ProcessConfig(in, tlsCfg)
		if err != nil {
			return nil, err
		}
		out.Debug = s.args.Debug || cfg.Debug
		out.EnableWebProfile = s.args.WebProfile || cfg.EnableWebProfile
		out.FromCommand = true
		out.AllowInsecureCreds = s.args.AllowInsecureCreds
		out.UntrustedEnv = s.args.UntrustedEnv
		out.PackagePath = path.Join(viamDotDir, "packages")
		return out, nil
	}

	processedConfig, err := processConfig(cfg)
	if err != nil {
		return err
	}
	if processedConfig.Cloud != nil {
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

	myRobot, err := robotimpl.New(ctx, processedConfig, s.logger, robotOptions...)
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		err = multierr.Combine(err, myRobot.Close(context.Background()))
	}()

	// watch for and deliver changes to the robot
	watcher, err := config.NewWatcher(ctx, cfg, s.logger)
	if err != nil {
		cancel()
		return err
	}
	defer func() {
		err = multierr.Combine(err, watcher.Close())
	}()
	onWatchDone := make(chan struct{})
	oldCfg := processedConfig
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
			case cfg := <-watcher.Config():
				processedConfig, err := processConfig(cfg)
				if err != nil {
					s.logger.Errorw("reconfiguration aborted: error processing config", "error", err)
					continue
				}

				// flag to restart web service if necessary
				diff, err := config.DiffConfigs(*oldCfg, *processedConfig, s.args.RevealSensitiveConfigDiffs)
				if err != nil {
					s.logger.Errorw("reconfiguration aborted: error diffing config", "error", err)
					continue
				}
				var options weboptions.Options

				if !diff.NetworkEqual {
					// TODO(RSDK-2694): use internal web service reconfiguration instead
					myRobot.StopWeb()
					options, err = s.createWebOptions(processedConfig)
					if err != nil {
						s.logger.Errorw("reconfiguration aborted: error creating weboptions", "error", err)
						continue
					}
				}

				myRobot.Reconfigure(ctx, processedConfig)

				if !diff.NetworkEqual {
					if err := myRobot.StartWeb(ctx, options); err != nil {
						s.logger.Errorw("reconfiguration failed: error starting web service while reconfiguring", "error", err)
					}
				}
				oldCfg = processedConfig
			}
		}
	}, func() {
		close(onWatchDone)
	})
	defer func() {
		<-onWatchDone
	}()
	defer cancel()

	options, err := s.createWebOptions(processedConfig)
	if err != nil {
		return err
	}
	return web.RunWeb(ctx, myRobot, options, s.logger)
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
	logger.Infof("%s, %s", message, traces)
	cancel()
}
