package tpspace

import (
	"testing"

	"go.viam.com/test"
)

var (
	defaultMMps   = 300.
	turnRadMeters = 0.3
)

func TestSim(t *testing.T) {
	simDist := 2500.
	alphaCnt := uint(121)
	for _, ptg := range defaultPTGs {
		radPS := defaultMMps / (turnRadMeters * 1000)

		ptgGen := ptg(defaultMMps, radPS)
		test.That(t, ptgGen, test.ShouldNotBeNil)
		grid, err := NewPTGGridSim(ptgGen, alphaCnt, simDist, false)
		test.That(t, err, test.ShouldBeNil)

		for i := uint(0); i < alphaCnt; i++ {
			traj, err := grid.Trajectory(index2alpha(i, alphaCnt), simDist)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, traj, test.ShouldNotBeNil)
		}
	}
}
