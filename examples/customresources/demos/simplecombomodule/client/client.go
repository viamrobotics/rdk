// Package main consumes the simplecombo composite over the SDK: one machine resource ("simplecombo")
// reachable under both the sensor and custom gizmo APIs at once. It is the client half of the
// simplecombomodule demo — start the server with `make run-module` in the parent directory (it serves
// the module via module.json on :8080), then run this with `make` here.
//
// This is the no-collision companion to the combomodule client: same composite-aware SDK helpers
// (resource.NamedFromProvider / resource.APIsOf and the per-API FromProvider calls), two APIs instead
// of three, and no colliding Properties method to disambiguate.
package main

import (
	"context"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := logging.NewLogger("simplecombo-client")

	ctx := context.Background()
	// Connect to the default localhost port for viam-server, as configured by module.json.
	machine, err := client.New(ctx, "localhost:8080", logger)
	if err != nil {
		logger.Fatal(err)
	}
	//nolint:errcheck
	defer machine.Close(ctx)

	// ── 1) ResourceNames: ONE short name, advertised under TWO APIs ─────────────────
	logger.Info("── ResourceNames (simplecombo should appear under 2 APIs) ──")
	for _, n := range machine.ResourceNames() {
		if n.Name == "simplecombo" {
			logger.Infof("  simplecombo advertised under: %s", n.API)
		}
	}

	// ── 2) api-less handle: ONE object that reports its full co-equal API set ───────
	logger.Info("── api-less composite handle ──")
	if combo, err := resource.NamedFromProvider(machine, "simplecombo"); err != nil {
		logger.Errorf("  NamedFromProvider: %v", err)
	} else {
		logger.Infof("  APIsOf(simplecombo): %v", resource.APIsOf(combo)) // [gizmo sensor]
	}

	// ── 3) as a SENSOR ──────────────────────────────────────────────────────────────
	logger.Info("── simplecombo as sensor ──")
	if s, err := sensor.FromProvider(machine, "simplecombo"); err != nil {
		logger.Errorf("  sensor.FromProvider: %v", err)
	} else if rd, err := s.Readings(ctx, nil); err != nil {
		logger.Errorf("  sensor Readings: %v", err)
	} else {
		logger.Infof("  sensor Readings: %v", rd) // map[reading:7]
	}

	// ── 4) as the CUSTOM gizmo API ───────────────────────────────────────────────────
	logger.Info("── simplecombo as gizmo (custom API) ──")
	if giz, err := gizmoapi.FromProvider(machine, "simplecombo"); err != nil {
		logger.Errorf("  gizmoapi.FromProvider: %v", err)
	} else {
		if ok, err := giz.DoOne(ctx, "simplecombo"); err != nil {
			logger.Errorf("  gizmo DoOne: %v", err)
		} else {
			logger.Infof("  gizmo DoOne(\"simplecombo\"): %v", ok) // true
		}
		if ok, err := giz.DoOne(ctx, "nope"); err != nil {
			logger.Errorf("  gizmo DoOne: %v", err)
		} else {
			logger.Infof("  gizmo DoOne(\"nope\"): %v", ok) // false
		}
		if s, err := giz.DoTwo(ctx, true); err != nil {
			logger.Errorf("  gizmo DoTwo: %v", err)
		} else {
			logger.Infof("  gizmo DoTwo(true): %q", s) // "arg1=true"
		}
	}

	logger.Info("── done ──")
}
