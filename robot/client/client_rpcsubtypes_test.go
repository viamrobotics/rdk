package client

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	gotestutils "go.viam.com/utils/testutils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/testutils/inject"
)

// TestWithoutRPCSubtypes covers RSDK-14364: a client that only uses compiled-in APIs can
// skip the descriptors, which is what keeps the CLI able to reach a slow machine. Asserts
// on the calls made rather than on timing, so it cannot flake.
func TestWithoutRPCSubtypes(t *testing.T) {
	logger := logging.NewTestLogger(t)

	var mu sync.Mutex
	var namesCalls, apisCalls int
	counts := func() (names, apis int) {
		mu.Lock()
		defer mu.Unlock()
		return namesCalls, apisCalls
	}

	injectRobot := &inject.Robot{}
	injectRobot.ResourceNamesFunc = func() []resource.Name {
		mu.Lock()
		defer mu.Unlock()
		namesCalls++
		return emptyResources
	}
	// server.ResourceRPCSubtypes is this function's only caller, so the count says whether
	// the client made the RPC.
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI {
		mu.Lock()
		defer mu.Unlock()
		apisCalls++
		return nil
	}
	injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	listener := gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	//nolint:errcheck
	go gServer.Serve(listener)
	defer gServer.Stop()

	// no background refresh or connection checks, so only the client-building calls count
	quiet := []RobotClientOption{WithRefreshEvery(0), WithCheckConnectedEvery(0)}

	t.Run("descriptors are fetched by default", func(t *testing.T) {
		namesBefore, apisBefore := counts()

		client, err := New(context.Background(), listener.Addr().String(), logger, quiet...)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		}()

		namesAfter, apisAfter := counts()
		test.That(t, namesAfter, test.ShouldBeGreaterThan, namesBefore)
		test.That(t, apisAfter, test.ShouldBeGreaterThan, apisBefore)
	})

	t.Run("WithoutRPCSubtypes skips them", func(t *testing.T) {
		namesBefore, apisBefore := counts()

		client, err := New(context.Background(), listener.Addr().String(), logger,
			append(quiet, WithoutRPCSubtypes())...)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		}()

		namesAfter, apisAfter := counts()
		// names are still needed; they are what callers use
		test.That(t, namesAfter, test.ShouldBeGreaterThan, namesBefore)
		test.That(t, apisAfter, test.ShouldEqual, apisBefore)
		test.That(t, client.ResourceRPCAPIs(), test.ShouldBeEmpty)
	})
}

// TestResourcesTimeoutOption guards the plumbing only. The budget cannot be exercised here
// because resourcesCallCtx never bounds under testing.Testing() (RSDK-5356); this just
// catches the wiring being dropped.
func TestResourcesTimeoutOption(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectRobot := &inject.Robot{}
	injectRobot.ResourceNamesFunc = func() []resource.Name { return emptyResources }
	injectRobot.ResourceRPCAPIsFunc = func() []resource.RPCAPI { return nil }
	injectRobot.MachineStatusFunc = func(context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{State: robot.StateRunning}, nil
	}

	listener := gotestutils.ReserveRandomListener(t)
	gServer := grpc.NewServer()
	pb.RegisterRobotServiceServer(gServer, server.New(injectRobot))
	//nolint:errcheck
	go gServer.Serve(listener)
	defer gServer.Stop()

	quiet := []RobotClientOption{WithRefreshEvery(0), WithCheckConnectedEvery(0)}

	t.Run("defaults when unset", func(t *testing.T) {
		client, err := New(context.Background(), listener.Addr().String(), logger, quiet...)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, client.resourcesTimeout, test.ShouldEqual, defaultResourcesTimeout)
	})

	t.Run("overridden by the option", func(t *testing.T) {
		client, err := New(context.Background(), listener.Addr().String(), logger,
			append(quiet, WithResourcesTimeout(42*time.Second))...)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, client.resourcesTimeout, test.ShouldEqual, 42*time.Second)
	})

	// zero would otherwise mean an already-expired context on every call
	t.Run("non-positive is ignored", func(t *testing.T) {
		client, err := New(context.Background(), listener.Addr().String(), logger,
			append(quiet, WithResourcesTimeout(0))...)
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			test.That(t, client.Close(context.Background()), test.ShouldBeNil)
		}()
		test.That(t, client.resourcesTimeout, test.ShouldEqual, defaultResourcesTimeout)
	})
}
