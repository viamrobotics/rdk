// Package main demonstrates driving an arm with the motion service's arm-streaming
// DoCommands (stream_start / stream_push / stream_flush / stream_abort / stream_status).
//
// It connects to a viam-server, starts a streaming session on an arm, pushes a smooth
// joint-space path in batches, polls the session status mid-stream, and then flushes
// (or, with -abort, aborts) the session.
//
// See services/motion/builtin/streaming/README.md for the protocol reference and
// examples/armstreaming/README.md for how to run this against a fake arm.
package main

import (
	"context"
	"errors"
	"flag"
	"math"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin"
)

func main() {
	host := flag.String("host", "localhost:8080", "address of the viam-server to connect to")
	armName := flag.String("arm", "arm1", "name of the arm to stream to")
	abort := flag.Bool("abort", false, "end the session with stream_abort instead of stream_flush")
	flag.Parse()

	logger := logging.NewLogger("armstreaming-client")
	ctx := context.Background()

	robot, err := client.New(ctx, *host, logger)
	if err != nil {
		logger.Fatal(err)
	}
	//nolint:errcheck
	defer robot.Close(ctx)

	motionService, err := motion.FromProvider(robot, "builtin")
	if err != nil {
		logger.Fatal(err)
	}

	// The session seeds its trajectory from the arm's current joint positions, so
	// generate a path that starts near them.
	a, err := arm.FromProvider(robot, *armName)
	if err != nil {
		logger.Fatal(err)
	}
	seed, err := a.JointPositions(ctx, nil)
	if err != nil {
		logger.Fatal(err)
	}

	// Start a session on the arm. The options shown are the defaults; omitting
	// "options" entirely gives the same behavior.
	resp, err := motionService.DoCommand(ctx, map[string]interface{}{
		builtin.DoStreamStart: map[string]interface{}{
			"arm": *armName,
			"options": map[string]interface{}{
				"target_runway_in_arm_ms":  100,
				"send_to_arm_interval_ms":  10,
				"vel_limit_deg_per_sec":    10,
				"accel_limit_deg_per_sec2": 10,
			},
		},
	})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("stream_start: %v", resp)

	// Push a smooth path -- one sinusoidal sweep of the first joint -- in batches.
	// A push blocks until the session has accepted every target in it, and pushes
	// must not overlap each other or the flush.
	const (
		numWaypoints = 100
		batchSize    = 10
		amplitudeRad = 0.2
	)
	for start := 0; start < numWaypoints; start += batchSize {
		batch := make([][]float64, 0, batchSize)
		for i := start; i < start+batchSize && i < numWaypoints; i++ {
			target := make([]float64, len(seed))
			for j, v := range seed {
				target[j] = float64(v)
			}
			target[0] += amplitudeRad * math.Sin(2*math.Pi*float64(i)/numWaypoints)
			batch = append(batch, target)
		}
		if _, err := motionService.DoCommand(ctx, map[string]interface{}{
			builtin.DoStreamPush: batch,
		}); err != nil {
			logger.Fatal(err)
		}
	}
	logger.Infof("pushed %d waypoints", numWaypoints)

	// Poll the session state mid-stream.
	resp, err = motionService.DoCommand(ctx, map[string]interface{}{builtin.DoStreamStatus: true})
	if err != nil {
		logger.Fatal(err)
	}
	logger.Infof("stream_status: %v", resp)

	endCmd := builtin.DoStreamFlush
	if *abort {
		endCmd = builtin.DoStreamAbort
	}

	// End the session. stream_flush drains the queued trajectory to the arm and waits
	// for the arm to finish executing it; stream_abort cancels immediately. Either way
	// the session outlives our request: if the per-call deadline expires first, the call
	// reports the session still running (as {"running": true} or a DeadlineExceeded
	// error, depending on timing) and the session keeps winding down on its own, so
	// repeat the command until it reports {"running": false}.
	for {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		resp, err = motionService.DoCommand(callCtx, map[string]interface{}{endCmd: true})
		cancel()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
				logger.Infof("%s still winding down; retrying", endCmd)
				continue
			}
			logger.Fatal(err)
		}
		logger.Infof("%s: %v", endCmd, resp)
		if running, _ := resp["running"].(bool); !running {
			break
		}
	}
	errMsg, hasErr := resp["error"].(string)
	switch {
	case !hasErr:
		logger.Info("session ended cleanly")
	case *abort && errMsg == context.Canceled.Error():
		// An aborted session always reports the cancellation as its error.
		logger.Info("session aborted")
	default:
		logger.Fatalf("session ended with error: %s", errMsg)
	}
}
