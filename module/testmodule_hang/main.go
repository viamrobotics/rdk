// Package main is a module whose process ignores SIGTERM/SIGINT. Used to test that
// SIGQUIT is sent to Golang modules and all goroutines are dumped into observable logs.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
)

func main() {
	// Swallow SIGTERM and SIGINT and log.
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for sig := range sigs {
			fmt.Fprintln(os.Stderr, "testmodule_hang: ignoring", sig)
		}
	}()

	ctx := context.Background()
	logger := module.NewLoggerFromArgs("TestModuleHang")

	mod, err := module.NewModuleFromArgs(ctx)
	if err != nil {
		logger.Fatalw("failed to create module from args", "error", err)
	}
	if err := mod.Start(ctx); err != nil {
		logger.Fatalw("failed to start module", "error", err)
	}

	// Serve a few distinctively-named goroutines so the eventual SIGQUIT dump has
	// identifiable stacks, then block the main goroutine indefinitely.
	var id int
	for range 3 {
		go hangingGoroutine(id, logger)
		id++
	}
	select {}
}

func hangingGoroutine(id int, logger logging.Logger) {
	logger.Debugw("testmodule_hang goroutine parked", "id", id)
	select {}
}
