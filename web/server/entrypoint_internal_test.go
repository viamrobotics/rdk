package server

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
)

func TestProcessConfigPropagatesDebugToModules(t *testing.T) {
	s := &robotServer{}

	// Debug set in the config: modules without an explicit LogLevel get "debug"
	// (forcing a restart with --log-level=debug); explicit levels are untouched.
	out, err := s.processConfig(&config.Config{
		Debug: true,
		Modules: []config.Module{
			{Name: "plain"},
			{Name: "preset", LogLevel: "info"},
		},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Modules[0].LogLevel, test.ShouldEqual, "debug")
	test.That(t, out.Modules[1].LogLevel, test.ShouldEqual, "info")

	// No debug anywhere: module configs are untouched.
	out, err = s.processConfig(&config.Config{
		Modules: []config.Module{{Name: "plain"}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Debug, test.ShouldBeFalse)
	test.That(t, out.Modules[0].LogLevel, test.ShouldEqual, "")

	// The -debug CLI flag propagates the same way.
	s = &robotServer{args: Arguments{Debug: true}}
	out, err = s.processConfig(&config.Config{
		Modules: []config.Module{{Name: "plain"}},
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.Debug, test.ShouldBeTrue)
	test.That(t, out.Modules[0].LogLevel, test.ShouldEqual, "debug")
}
