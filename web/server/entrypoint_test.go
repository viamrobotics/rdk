// Package server implements the entry point for running a robot web server.
package server_test

import (
	"context"
	"runtime"
	"strconv"
	"testing"

	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
)

// numResources is the # of resources in /etc/configs/fake.json + the 2
// expected builtin resources.
const numResources = 20

func TestNumResources(t *testing.T) {
	if runtime.GOARCH == "arm" {
		t.Skip("skipping on 32-bit ARM, subprocess build warnings cause failure")
	}
	logger := logging.NewTestLogger(t)
	cfgFilename := utils.ResolveFile("/etc/configs/fake.json")
	cfg, err := config.Read(context.Background(), cfgFilename, logger)
	test.That(t, err, test.ShouldBeNil)

	p, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)

	port := strconv.Itoa(p)
	cfg.Network.BindAddress = ":" + port
	cfgFilename, err = robottestutils.MakeTempConfig(t, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	serverPath, err := testutils.BuildTempModule(t, "web/cmd/server/")
	test.That(t, err, test.ShouldBeNil)

	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name: serverPath,
		Args: []string{"-config", cfgFilename},
		CWD:  utils.ResolveFile("./"),
		Log:  true,
	}, logger.AsZap())

	err = server.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	conn, err := robottestutils.Connect(port)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()
	rc := robotpb.NewRobotServiceClient(conn)

	resourceNames, err := rc.ResourceNames(context.Background(), &robotpb.ResourceNamesRequest{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resourceNames.Resources), test.ShouldEqual, numResources)
}
