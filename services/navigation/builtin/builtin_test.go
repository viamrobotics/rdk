// Package builtin contains the default navigation service, along with a gRPC server and client
package builtin

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	_ "go.viam.com/rdk/components/base/fake"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	robotimpl "go.viam.com/rdk/robot/impl"
	_ "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/test"
)

func TestReadCfg(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, "../data/nav_cfg.json", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	_, err = navigation.FromRobot(myRobot, "test_navigation")
	test.That(t, err, test.ShouldBeNil)
}
