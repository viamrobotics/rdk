// Package main consumes the combodevice composite over the SDK: one machine resource ("combo")
// reachable under the camera, movement_sensor, and custom gizmo APIs at once. It is the client half of
// the combomodule demo — start the server with `make run-module` in the parent directory (it serves the
// module via module.json on :8080), then run this with `make` here.
//
// Because it builds against this (composite) RDK, the composite-aware SDK helpers
// resource.NamedFromProvider / resource.APIsOf are available alongside the per-API FromProvider calls.
package main

import (
	"context"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
)

func main() {
	logger := logging.NewLogger("combo-client")

	ctx := context.Background()
	// Connect to the default localhost port for viam-server, as configured by module.json.
	machine, err := client.New(ctx, "localhost:8080", logger)
	if err != nil {
		logger.Fatal(err)
	}
	//nolint:errcheck
	defer machine.Close(ctx)

	// ── 1) ResourceNames: ONE short name, advertised under THREE APIs ───────────────
	logger.Info("── ResourceNames (combo should appear under 3 APIs) ──")
	for _, n := range machine.ResourceNames() {
		if n.Name == "combo" {
			logger.Infof("  combo advertised under: %s", n.API)
		}
	}

	// ── 2) api-less handle: ONE object that reports its full co-equal API set ───────
	logger.Info("── api-less composite handle ──")
	if combo, err := resource.NamedFromProvider(machine, "combo"); err != nil {
		logger.Errorf("  NamedFromProvider: %v", err)
	} else {
		logger.Infof("  APIsOf(combo): %v", resource.APIsOf(combo)) // [gizmo camera movement_sensor]
	}

	// ── 3) as a CAMERA: the colliding Properties method, camera side ────────────────
	logger.Info("── combo as camera ──")
	if cam, err := camera.FromProvider(machine, "combo"); err != nil {
		logger.Errorf("  camera.FromProvider: %v", err)
	} else if props, err := cam.Properties(ctx); err != nil {
		logger.Errorf("  camera Properties: %v", err)
	} else {
		logger.Infof("  camera Properties: %+v", props) // FrameRate=30 SupportsPCD=true
	}

	// ── 4) as a MOVEMENT SENSOR: SAME method name, DIFFERENT facade & return type ───
	logger.Info("── combo as movement_sensor ──")
	if ms, err := movementsensor.FromProvider(machine, "combo"); err != nil {
		logger.Errorf("  movementsensor.FromProvider: %v", err)
	} else {
		if props, err := ms.Properties(ctx, nil); err != nil {
			logger.Errorf("  movementsensor Properties: %v", err)
		} else {
			logger.Infof("  movementsensor Properties: %+v", props) // AngularVelocitySupported=true
		}
		if rd, err := ms.Readings(ctx, nil); err != nil {
			logger.Errorf("  movementsensor Readings: %v", err)
		} else {
			logger.Infof("  movementsensor Readings: %v", rd) // map[reading:7]
		}
		if pt, alt, err := ms.Position(ctx, nil); err != nil {
			logger.Errorf("  movementsensor Position: %v", err)
		} else {
			logger.Infof("  movementsensor Position: lat=%v lng=%v alt=%v", pt.Lat(), pt.Lng(), alt)
		}
	}

	// ── 5) as the CUSTOM gizmo API: composites span builtin AND module-defined APIs ──
	logger.Info("── combo as gizmo (custom API) ──")
	if giz, err := gizmoapi.FromProvider(machine, "combo"); err != nil {
		logger.Errorf("  gizmoapi.FromProvider: %v", err)
	} else {
		if ok, err := giz.DoOne(ctx, "combo"); err != nil {
			logger.Errorf("  gizmo DoOne: %v", err)
		} else {
			logger.Infof("  gizmo DoOne(\"combo\"): %v", ok) // true
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
