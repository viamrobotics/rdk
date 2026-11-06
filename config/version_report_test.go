package config

import (
	"context"
	"fmt"
	"testing"
	"time"

	pb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/durationpb"

	"go.viam.com/rdk/config/testutils"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
)

func TestConfigRequestReportsVersions(t *testing.T) {
	const robotPartID = "forVersionReportTest"
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	clearCache(robotPartID)
	defer clearCache(robotPartID)

	fakeServer, cleanup := testutils.NewFakeCloudServer(t, ctx, logger)
	defer cleanup()

	cloud := &Cloud{
		ID:                robotPartID,
		Secret:            testutils.FakeCredentialPayLoad,
		FQDN:              "fqdn",
		LocalFQDN:         "localFqdn",
		SignalingInsecure: true,
	}
	cloudConfProto, err := CloudConfigToProto(cloud)
	test.That(t, err, test.ShouldBeNil)
	fakeServer.StoreDeviceConfig(robotPartID, &pb.RobotConfig{Cloud: cloudConfProto}, nil)

	appAddress := fmt.Sprintf("http://%s", fakeServer.Addr().String())
	prevCloudCfg := *cloud
	prevCloudCfg.AppAddress = appAddress
	appConn, err := grpc.NewAppConn(ctx, appAddress, robotPartID, prevCloudCfg.GetCloudCredsDialOpt(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer appConn.Close()

	t.Run("viam_server_uptime is measured from process start", func(t *testing.T) {
		before := time.Since(processStartedAt)
		_, err := readFromCloud(ctx, robotPartID, &prevCloudCfg, false, logger, appConn, nil)
		test.That(t, err, test.ShouldBeNil)
		after := time.Since(processStartedAt)

		info := fakeServer.LastAgentInfo()
		test.That(t, info, test.ShouldNotBeNil)
		test.That(t, info.ViamServerUptime, test.ShouldNotBeNil)
		uptime := info.ViamServerUptime.AsDuration()
		test.That(t, uptime, test.ShouldBeGreaterThanOrEqualTo, before)
		test.That(t, uptime, test.ShouldBeLessThanOrEqualTo, after)
		test.That(t, info.ModuleVersions, test.ShouldBeEmpty)
	})

	t.Run("module versions come from the provider", func(t *testing.T) {
		want := []*pb.ModuleVersionInfo{
			{ModuleId: "viam:raspberry-pi", RunningVersion: "1.2.0", RunningFor: durationpb.New(time.Minute)},
			{ModuleId: "viam:realsense"},
		}
		calls := 0
		provider := func() []*pb.ModuleVersionInfo {
			calls++
			return want
		}

		_, err := readFromCloud(ctx, robotPartID, &prevCloudCfg, false, logger, appConn, provider)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, calls, test.ShouldEqual, 1)

		info := fakeServer.LastAgentInfo()
		test.That(t, info, test.ShouldNotBeNil)
		test.That(t, info.ModuleVersions, test.ShouldHaveLength, 2)
		test.That(t, info.ModuleVersions[0], test.ShouldResembleProto, want[0])
		test.That(t, info.ModuleVersions[1], test.ShouldResembleProto, want[1])
		test.That(t, info.ViamServerUptime, test.ShouldNotBeNil)
	})
}

func TestModuleEqualsIgnoresPackageVersion(t *testing.T) {
	a := Module{Name: "m", ExePath: "/a", Type: ModuleTypeRegistry, ModuleID: "viam:m", PackageVersion: "1.0.0"}
	b := a
	b.PackageVersion = "2.0.0"
	test.That(t, a.Equals(b), test.ShouldBeTrue)

	b.ExePath = "/b"
	test.That(t, a.Equals(b), test.ShouldBeFalse)
}
