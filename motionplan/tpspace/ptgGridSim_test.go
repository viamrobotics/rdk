package tpspace

import (
	"testing"

	"go.viam.com/test"
)

const (
	turnRadMeters = 0.3
	resolution    = 20
)

func TestSim(t *testing.T) {
	simDist := 2500.
	alphaCnt := uint(121)
	for _, ptg := range defaultPTGs {
		ptgGen := ptg(turnRadMeters)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		grid, err := NewPTGGridSim(ptgGen, alphaCnt, simDist, false)
		test.That(t, err, test.ShouldBeNil)

		for i := uint(0); i < alphaCnt; i++ {
			traj, err := grid.Trajectory(index2alpha(i, alphaCnt), 0, simDist, resolution)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, traj, test.ShouldNotBeNil)
		}
	}
}
