package tpspace

import(
	"testing"
	"fmt"
	
	//~ rutils "go.viam.com/rdk/utils"
)

var defaultPTGs = []func(float64, float64, float64) PrecomputePTG {
	NewCirclePTG,
	NewCCPTG,
	NewCCSPTG,
	NewCSPTG,
	NewAlphaPTG,
}

var defaultMps = 1.
var defaultDps = 45.

func TestSim(t *testing.T) {
	fmt.Println("type,X,Y")
	
	
	for _, ptg := range defaultPTGs {
		
		ptgGen := ptg(defaultMps, defaultDps, 1.)
		ptgPrecomp, _ := NewPTGGridSim(ptgGen, defaultAlphaCnt)

		

		for i := uint(0); i < defaultAlphaCnt; i++ {
			for _, x := range ptgPrecomp.Trajectory(i) {
				pt := x.Pose.Point()
				fmt.Printf("FINALPATH,%f,%f\n", pt.X, pt.Y)
			}
		}
		ptgGen = ptg(defaultMps, defaultDps, -1.)
		if ptgGen != nil {
			ptgPrecomp, _ := NewPTGGridSim(ptgGen, defaultAlphaCnt)

			

			for i := uint(0); i < defaultAlphaCnt; i++ {
				for _, x := range ptgPrecomp.Trajectory(i) {
					pt := x.Pose.Point()
					fmt.Printf("FINALPATH,%f,%f\n", pt.X, pt.Y)
				}
			}
		}
	}
}
