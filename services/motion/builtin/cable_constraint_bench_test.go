package builtin

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/encoding/protojson"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

// BenchmarkDoCommandPlan measures a DoCommand "plan" call under three settings:
//   - no_lookup: cable_constraint is false in the config, so armsCarryingCameras is skipped.
//   - lookup_only: the config leaves it unset so armsCarryingCameras runs, but the request sets
//     cable_constraint false, so the constraint itself stays off.
//   - lookup_and_constraint: the shipped default; the lookup runs and, because a camera is mounted
//     on the arm, the guess turns the constraint on.
//
// The difference between no_lookup and lookup_only is the overhead of armsCarryingCameras alone.
func BenchmarkDoCommandPlan(b *testing.B) {
	ctx := context.Background()
	logger := logging.NewTestLogger(b)
	cfg, err := config.Read(ctx, "../data/moving_arm.json", logger, nil)
	test.That(b, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, nil, logger)
	test.That(b, err, test.ShouldBeNil)
	defer myRobot.Close(ctx)

	svc, err := motion.FromProvider(myRobot, "builtin")
	test.That(b, err, test.ShouldBeNil)
	builtinSvc, ok := svc.(*builtIn)
	test.That(b, ok, test.ShouldBeTrue)

	box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{1000, 1000, 1000}), r3.Vector{1, 1, 1}, "box")
	test.That(b, err, test.ShouldBeNil)
	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame("world", []spatialmath.Geometry{box})}, nil,
	)
	test.That(b, err, test.ShouldBeNil)

	planCommand := func(extra map[string]interface{}) map[string]interface{} {
		moveReq := motion.MoveReq{
			ComponentName: "pieceGripper",
			WorldState:    worldState,
			Destination:   referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50})),
			Extra:         extra,
		}
		proto, err := moveReq.ToProto(svc.Name().Name)
		test.That(b, err, test.ShouldBeNil)
		bytes, err := protojson.Marshal(proto)
		test.That(b, err, test.ShouldBeNil)
		command, err := protoutils.StructToStructPb(map[string]interface{}{DoPlan: string(bytes)})
		test.That(b, err, test.ShouldBeNil)

		return command.AsMap()
	}

	disabled := false
	cases := []struct {
		name  string
		conf  *bool
		extra map[string]interface{}
	}{
		{name: "no_lookup", conf: &disabled},
		{name: "lookup_only", extra: map[string]interface{}{"cable_constraint": false}},
		{name: "lookup_and_constraint"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			builtinSvc.mu.Lock()
			builtinSvc.conf.CableConstraint = tc.conf
			builtinSvc.mu.Unlock()
			command := planCommand(tc.extra)

			b.ResetTimer()
			for range b.N {
				_, err := svc.DoCommand(ctx, command)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkArmsCarryingCameras measures armsCarryingCameras on its own against a frame system of
// about 100 frames. Two arms each carry a gripper and a camera; the remaining frames are filler
// mounted on the world, either non-camera components or fixed cameras (which each cost a frame
// traceback even though no arm carries them).
func BenchmarkArmsCarryingCameras(b *testing.B) {
	const totalFrames = 100
	fillers := []struct {
		name string
		make func(name string) resource.Resource
	}{
		{name: "filler_grippers", make: func(name string) resource.Resource { return inject.NewGripper(name) }},
		{name: "filler_fixed_cameras", make: func(name string) resource.Resource { return inject.NewCamera(name) }},
	}

	for _, filler := range fillers {
		b.Run(filler.name, func(b *testing.B) {
			frameSystem := referenceframe.NewEmptyFrameSystem("bench")
			components := map[string]resource.Resource{}
			addStatic := func(name string, parent referenceframe.Frame, component resource.Resource) referenceframe.Frame {
				frame, err := referenceframe.NewStaticFrame(name, spatialmath.NewZeroPose())
				test.That(b, err, test.ShouldBeNil)
				test.That(b, frameSystem.AddFrame(frame, parent), test.ShouldBeNil)
				components[name] = component

				return frame
			}

			for armIdx := range 2 {
				armName := fmt.Sprintf("arm%d", armIdx)
				arm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), armName)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, frameSystem.AddFrame(arm, frameSystem.World()), test.ShouldBeNil)
				components[armName] = inject.NewArm(armName)

				gripper := addStatic(armName+"-gripper", arm, inject.NewGripper(armName+"-gripper"))
				addStatic(armName+"-cam", gripper, inject.NewCamera(armName+"-cam"))
			}
			for fillerIdx := len(frameSystem.FrameNames()) - 1; fillerIdx < totalFrames; fillerIdx++ {
				name := fmt.Sprintf("filler%d", fillerIdx)
				addStatic(name, frameSystem.World(), filler.make(name))
			}

			b.ResetTimer()
			for range b.N {
				if arms := armsCarryingCameras(frameSystem, components); len(arms) != 2 {
					b.Fatalf("expected two camera arms, got %v", arms)
				}
			}
		})
	}
}
