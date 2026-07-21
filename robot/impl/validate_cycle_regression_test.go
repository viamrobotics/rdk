package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
	rutils "go.viam.com/rdk/utils"
)

// TestModularValidateFailureThenRecovery drives a modular resource through a
// valid -> invalid -> valid config cycle, where the middle config fails the
// module's Validate.
func TestModularValidateFailureThenRecovery(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	modPath := rtestutils.BuildTempModule(t, "module/validatecycle")
	model := resource.NewModel("rdk", "test", "validatecycle")

	mkCfg := func(bad bool) *config.Config {
		return &config.Config{
			Modules: []config.Module{{Name: "vcmod", ExePath: modPath}},
			Components: []resource.Config{{
				Name:       "thingy",
				API:        generic.API,
				Model:      model,
				Attributes: rutils.AttributeMap{"bad": bad},
			}},
		}
	}
	r := setupLocalRobot(t, ctx, mkCfg(false), logger, WithDisableCompleteConfigWorker())

	// Healthy start.
	_, err := r.ResourceByName(generic.Named("thingy"))
	test.That(t, err, test.ShouldBeNil)

	// Push a config that fails the module's Validate: the resource becomes
	// unavailable
	r.Reconfigure(ctx, mkCfg(true))
	_, err = r.ResourceByName(generic.Named("thingy"))
	test.That(t, err, test.ShouldNotBeNil)

	// Push the corrected config: the resource must come back.
	r.Reconfigure(ctx, mkCfg(false))
	res, err := r.ResourceByName(generic.Named("thingy"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	// The recovered resource must actually work.
	resp, err := res.DoCommand(ctx, map[string]interface{}{"ping": true})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["ok"], test.ShouldEqual, true)
}
