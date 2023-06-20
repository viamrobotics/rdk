package operation

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
	"go.viam.com/test"
)

func ConfigFromFile(t *testing.T, filePath string) *config.Config {
	t.Helper()
	logger := golog.NewTestLogger(t)
	buf, err := ioutil.ReadFile(filePath)
	test.That(t, err, test.ShouldBeNil)
	conf, err := config.FromReader(context.Background(), filePath, bytes.NewReader(buf), logger)
	test.That(t, err, test.ShouldBeNil)
	return conf
}

func NoErr[ResultT any](t *testing.T, res ResultT, err error) ResultT {
	test.That(t, err, test.ShouldBeNil)
	return res
}

func TestCancelDependentOps(t *testing.T) {
	// logger := golog.NewTestLogger(t)
	// conf := ConfigFromFile(t, "./data/motor_base_robot.json")
	// mockAPI := resource.APINamespaceRDK.WithComponentType("mock")
	//
	// ctx := context.Background()
	// robot := NoErr(t, rimpl.New(ctx, conf, logger))
}
