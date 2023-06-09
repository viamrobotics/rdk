// Package server implements the entry point for running a robot web server.
package server_test

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/edaniels/golog"

	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/testutils/robottestutils"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/pexec"
)

const numResources = 19

func TestOpID(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfgFilename := "/Users/bashareid/Developer/rdk/etc/configs/fake.json"
	cfg, err := config.Read(context.Background(), cfgFilename, logger)
	test.That(t, err, test.ShouldBeNil)

	p, err := goutils.TryReserveRandomPort()
	test.That(t, err, test.ShouldBeNil)

	port := strconv.Itoa(p)
	cfg.Network.BindAddress = ":" + port
	cfgFilename, err = makeTempConfig(t, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	server := pexec.NewManagedProcess(pexec.ProcessConfig{
		Name: "bash",
		Args: []string{"-c", "make server && exec bin/`uname`-`uname -m`/viam-server -config " + cfgFilename},
		CWD:  utils.ResolveFile("./"),
		Log:  true,
	}, logger)

	err = server.Start(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, server.Stop(), test.ShouldBeNil)
	}()

	rc, _, conn, err := robottestutils.Connect(port)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, conn.Close(), test.ShouldBeNil)
	}()

	resourceNames, err := rc.ResourceNames(context.Background(), &robotpb.ResourceNamesRequest{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(resourceNames.Resources), test.ShouldEqual, numResources)
}

func makeTempConfig(t *testing.T, cfg *config.Config, logger golog.Logger) (string, error) {
	if err := cfg.Ensure(false, logger); err != nil {
		return "", err
	}
	output, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	file, err := os.CreateTemp(t.TempDir(), "fake-*")
	if err != nil {
		return "", err
	}
	_, err = file.Write(output)
	if err != nil {
		return "", err
	}
	return file.Name(), file.Close()
}
