package motionplan

import (
	"errors"
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// BenchmarkCableConstraintCheck measures the function returned by newCableConstraintCheck. A
// passing state scans every band; the violating state is out of range only on the last joint of
// the last arm, so it also scans every band before failing and pays for formatting the error.
func BenchmarkCableConstraintCheck(b *testing.B) {
	for _, armCount := range []int{1, 2, 10} {
		b.Run(fmt.Sprintf("arms=%d", armCount), func(b *testing.B) {
			frameSystem := referenceframe.NewEmptyFrameSystem("bench")
			start := referenceframe.NewLinearInputs()
			var lastArm string
			for armIdx := range armCount {
				lastArm = fmt.Sprintf("arm%d", armIdx)
				model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/kinematics/ur5e.json"), lastArm)
				test.That(b, err, test.ShouldBeNil)
				test.That(b, frameSystem.AddFrame(model, frameSystem.World()), test.ShouldBeNil)
				start.Put(lastArm, make([]referenceframe.Input, len(model.DoF())))
			}

			check, err := newCableConstraintCheck(frameSystem, start, CableConstraint{})
			test.That(b, err, test.ShouldBeNil)
			test.That(b, check, test.ShouldNotBeNil)

			passing := &StateFS{Configuration: start, FS: frameSystem}
			violatingInputs := make([]referenceframe.Input, 6)
			violatingInputs[5] = utils.DegToRad(190)
			violatingConfiguration := start.Copy()
			violatingConfiguration.Put(lastArm, violatingInputs)
			violating := &StateFS{Configuration: violatingConfiguration, FS: frameSystem}

			b.Run("passing", func(b *testing.B) {
				for range b.N {
					if err := check(passing); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("violating", func(b *testing.B) {
				for range b.N {
					if err := check(violating); !errors.Is(err, ErrCableConstraintViolated) {
						b.Fatalf("expected a cable violation, got %v", err)
					}
				}
			})
		})
	}
}
