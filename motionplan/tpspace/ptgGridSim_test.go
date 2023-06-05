package tpspace

import(
	"testing"
	"fmt"
	
	//~ rutils "go.viam.com/rdk/utils"
)

func TestSim(t *testing.T) {
	
	precompPTG := &simPtgAlpha{maxMps,maxDps}
	
	ptg, _ := NewPTGGridSim(2., 135., precompPTG)
	for _, x := range ptg.precomputeTraj[0][0] {
		fmt.Println(*x)
	}
}
