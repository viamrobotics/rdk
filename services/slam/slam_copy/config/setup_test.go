package config

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func setupTestGRPCServer(tb testing.TB) (*grpc.Server, int) {
	listener, err := net.Listen("tcp", ":0")
	test.That(tb, err, test.ShouldBeNil)
	grpcServer := grpc.NewServer()
	go grpcServer.Serve(listener)

	return grpcServer, listener.Addr().(*net.TCPAddr).Port
}

func TestGRPCConnection(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("Invalid grpc connection", func(t *testing.T) {
		port := "invalid_unused_port:0"
		_, _, err := SetupGRPCConnection(context.Background(), port, 1, logger)
		test.That(t, err, test.ShouldBeError, errors.New("context deadline exceeded"))
	})
	t.Run("Valid grpc connection", func(t *testing.T) {
		// Setup grpc server and attempt to connect to that one
		grpcServer, portNum := setupTestGRPCServer(t)
		defer grpcServer.Stop()
		port := fmt.Sprintf(":%d", portNum)
		_, _, err := SetupGRPCConnection(context.Background(), port, 1, logger)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestSetupDirectories(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("Valid directories", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "*")
		defer os.RemoveAll(tempDir)
		test.That(t, err, test.ShouldBeNil)
		err = SetupDirectories(tempDir, logger)
		test.That(t, err, test.ShouldBeNil)
		// Ensure that all of the directories have been created
		_, errData := os.Stat(tempDir + "/data")
		test.That(t, errData, test.ShouldBeNil)
		_, errMap := os.Stat(tempDir + "/map")
		test.That(t, errMap, test.ShouldBeNil)
		_, errConfig := os.Stat(tempDir + "/config")
		test.That(t, errConfig, test.ShouldBeNil)
		// Ensure that the tests work
		_, errFoo := os.Stat(tempDir + "/foodir")
		test.That(t, errFoo, test.ShouldBeError)
	})
	t.Run("Invalid permissions", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "*")
		test.That(t, err, test.ShouldBeNil)
		defer os.RemoveAll(tempDir)
		noPermsDir := tempDir + "/no_permissions"
		// create a directory in the temp folder
		// that doesn't have write permissions
		// in rwx format: --- --- ---
		err = os.Mkdir(noPermsDir, 0o000)
		test.That(t, err, test.ShouldBeNil)
		err = SetupDirectories(noPermsDir, logger)
		test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "issue creating directory at")
	})
}
