// Command _local_wss_test plays a one-shot motion demo against a locally running viam-server to show the
// configured world_state_store service feeding obstacles into the motion service. It has two modes, one
// per fake sensor type (depth_camera, lidar); pick the mode that matches the store's input_sensor_type
// in your machine config. Each run plays a single scripted path (good for recording), then exits.
// Throwaway; the _-prefixed dir is ignored by `go build ./...`.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/golang/geo/r3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	robotclient "go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/spatialmath"
)

// waypoint is one step of the path. blocked marks targets that sit inside an obstacle, so a correct
// motion service refuses them and the arm holds its previous pose.
type waypoint struct {
	label   string
	x, y, z float64
	blocked bool
}

// demos maps each fake sensor mode to a path suited to that obstacle layout. Clear targets make the arm
// swing visibly; blocked targets prove the store's obstacles are respected. The order reads as a single
// continuous sweep for recording.
var demos = map[string][]waypoint{
	// depth_camera: three 150mm blocks in a row at x=-250/0/250, y=400, z=75.
	"depth_camera": {
		{"start: off to the side", 400, -200, 400, false},
		{"approach the block row", 400, 150, 350, false},
		{"try to dip into blue block", 250, 400, 90, true},
		{"rise and cross above the blocks", 0, 350, 500, false},
		{"try to dip into red block", -250, 400, 90, true},
		{"settle on the left", -400, 150, 350, false},
	},
	// lidar: a ring of surfaces at radius 500 on bearings 30/90/150/210/270/330; gaps at 0/60/.../300.
	"lidar": {
		{"start: high on +x", 400, 0, 400, false},
		{"reach through the +x gap", 450, 0, 220, false},
		{"try to reach the 30deg surface", 433, 250, 150, true},
		{"thread the 60deg gap", 225, 390, 250, false},
		{"try to reach the 90deg surface", 0, 500, 150, true},
		{"thread the 120deg gap", -225, 390, 250, false},
	},
}

func main() {
	mode := flag.String("mode", "depth_camera", "which path to play: depth_camera or lidar (match the store's input_sensor_type)")
	address := flag.String("address", envOr("VIAM_ADDRESS", "localhost:8080"), "machine address to dial (the machine's .viam.cloud address, or localhost:PORT)")
	apiKeyID := flag.String("api-key-id", os.Getenv("VIAM_API_KEY_ID"), "machine API key id (cloud.api_key.id from the downloaded config)")
	apiKey := flag.String("api-key", os.Getenv("VIAM_API_KEY"), "machine API key (cloud.api_key.key from the downloaded config)")
	pause := flag.Duration("pause", 1500*time.Millisecond, "pause between moves")
	flag.Parse()

	path, ok := demos[*mode]
	if !ok {
		fmt.Printf("unknown -mode %q; use depth_camera or lidar\n", *mode)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	logger := logging.NewLogger("wsstest")

	// A cloud machine (running the downloaded config) requires auth: pass the API key, exactly as the
	// visualizer does. Without a key we fall back to an insecure local dial, which only works against a
	// cloudless viam-server (no cloud block in its config).
	var dialOpt rpc.DialOption
	if *apiKeyID != "" && *apiKey != "" {
		dialOpt = rpc.WithEntityCredentials(*apiKeyID, rpc.Credentials{Type: rpc.CredentialsTypeAPIKey, Payload: *apiKey})
	} else {
		logger.Info("no -api-key-id/-api-key given; dialing insecurely (works only against a cloudless viam-server)")
		dialOpt = rpc.WithInsecure()
	}

	// Bound the dial so a wrong address or missing credentials fails fast instead of hanging forever.
	dialCtx, cancelDial := context.WithTimeout(ctx, 30*time.Second)
	defer cancelDial()
	machine, err := robotclient.New(dialCtx, *address, logger, robotclient.WithDialOptions(dialOpt))
	if err != nil {
		logger.Fatalf("could not connect to %q: %v", *address, err)
	}
	defer func() { _ = machine.Close(context.Background()) }()

	// Show what the store is serving so it is obvious the mode matches the configured sensor.
	if wss, err := worldstatestore.FromRobot(machine, "worldstate"); err == nil {
		if uuids, err := wss.ListUUIDs(ctx, nil); err == nil {
			fmt.Printf("world state store serves %d transform(s); playing the %q path\n", len(uuids), *mode)
		}
	}

	ms, err := motion.FromRobot(machine, "builtin")
	if err != nil {
		logger.Fatal(err)
	}

	for _, wp := range path {
		if ctx.Err() != nil {
			fmt.Println("\ninterrupted, exiting")
			return
		}
		dest := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: wp.x, Y: wp.y, Z: wp.z}))
		moved, err := ms.Move(ctx, motion.MoveReq{ComponentName: "arm", Destination: dest})
		fmt.Printf("  %-34s -> %s\n", wp.label, outcome(wp, moved, err))
		select {
		case <-time.After(*pause):
		case <-ctx.Done():
			fmt.Println("\ninterrupted, exiting")
			return
		}
	}
	fmt.Println("done")
}

// envOr returns the environment variable value if set, otherwise the fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// outcome classifies a move result against the waypoint's expectation for readable demo output.
func outcome(wp waypoint, moved bool, err error) string {
	switch {
	case wp.blocked && !moved:
		return "correctly refused (obstacle respected)"
	case wp.blocked && moved:
		return "UNEXPECTED: moved into the obstacle!"
	case !wp.blocked && moved:
		return "moved"
	default:
		return fmt.Sprintf("unexpected failure: %v", err)
	}
}
