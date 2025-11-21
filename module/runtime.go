package module

import (
	"context"
	"os"
	"runtime/debug"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
)

// ModularMain can be called as the main function from a module. It will start up a module with all
// the provided APIModels added to it.
func ModularMain(models ...resource.APIModel) {
	mainWithArgs := func(ctx context.Context, args []string, logger logging.Logger) error {
		info, ok := debug.ReadBuildInfo()
		if ok {
			logger.Infof("module version: %s, go version: %s", info.Main.Version, info.GoVersion)
		}

		mod, err := NewModuleFromArgs(ctx)
		if err != nil {
			return err
		}

		for _, apiModel := range models {
			if err = mod.AddModelFromRegistry(ctx, apiModel.API, apiModel.Model); err != nil {
				return err
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if os.Getenv(NoModuleParentEnvVar) != "true" {
			// Register a connection change handler so that if the connection back to the
			// viam-server fails, we try to reconnect but cancels the context
			// if reconnection fails. A connection failure usually means that the
			// viam-server has exited, so keeping the module process alive is not useful.
			// Even if the viam-server is still alive, there is no reason to keep a
			// module alive if it can no longer talk to the parent viam-server.
			//
			// On systems with SIGPIPE such as Unix, this handler is unnecessary as
			// [utils.ContextualMainWithSIGPIPE] will catch the SIGPIPE and cancel the context.
			// On systems without SIGPIPE such as Windows, this is necessary to make sure
			// module process are not leaked when the viam-server exits unexpectedly.
			//
			// If the viam-server is truly still alive, it is expected that viam-server
			// will eventually restart the module through the OnUnexpectedExit handler.
			mod.RegisterParentConnectionChangeHandler(func(rc *client.RobotClient) {
				if !rc.Connected() {
					// If viam-server is alive, these logs can be used to debug.
					// Otherwise, these logs will not go anywhere since the logger will
					// only attempt to write to the dead viam-server.
					logger.Info("connection to viam-server lost; attempting to reconnect")
					if err := rc.Connect(ctx); err != nil {
						logger.Info("reconnect attempt failed; shutting down module")
						cancel()
					}
				}
			})
		}

		err = mod.Start(ctx)
		defer mod.Close(ctx)
		if err != nil {
			return err
		}

		<-ctx.Done()
		return nil
	}

	// On systems with SIGPIPE such as Unix, using SIGPIPE to signal a module shutdown
	// is preferred as it is more responsive.
	utils.ContextualMainWithSIGPIPE(mainWithArgs, NewLoggerFromArgs(""))
}
