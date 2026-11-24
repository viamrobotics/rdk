// Package main is a module that never serves gRPC and exits with an error shortly after
// starting. Used to test a module that crashes before it can answer a ready request.
package main

import (
	"fmt"
	"net"
	"os"
	"slices"
	"time"
)

func main() {
	// viam-server does not reserve a TCP-mode module's port between choosing it and the module
	// binding it. Hold the port so that no unrelated gRPC server can answer the ready request
	// in the module's place. In unix socket mode nothing is bound: the socket file must stay
	// absent for viam-server to treat the module as not yet started.
	var lis net.Listener
	if slices.Contains(os.Args[1:], "--tcp-mode") {
		// Matches rutils.ViamModuleAddress.
		addr := os.Getenv("VIAM_MODULE_ADDRESS")

		var err error
		//nolint: noctx
		lis, err = net.Listen("tcp", addr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "testmodule_crash: failed to listen:", err)
			os.Exit(2)
		}
	}

	time.Sleep(2 * time.Second)
	if lis != nil {
		if err := lis.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "testmodule_crash: failed to close listener:", err)
		}
	}
	os.Exit(1)
}
