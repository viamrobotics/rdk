package trossen

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestBadConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.ReadLocalConfig(context.Background(), "bad_config.json", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(conf.Components), test.ShouldEqual, 1)
	compConf := conf.Components[0]
	_, err = compConf.Validate("")
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGoodConfig(t *testing.T) {
	logger := golog.NewTestLogger(t)
	conf, err := config.ReadLocalConfig(context.Background(), "good_config.json", logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(conf.Components), test.ShouldEqual, 1)
	compConf := conf.Components[0]
	deps, err := compConf.Validate("")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, deps, test.ShouldResemble, []string{"gripper_arm"})
}
