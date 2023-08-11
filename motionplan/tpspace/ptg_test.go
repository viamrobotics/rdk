package tpspace

import (
	"testing"
	//~ "fmt"

	"go.viam.com/test"
)

func TestAlphaIdx(t *testing.T) {
	for i := uint(0); i < defaultAlphaCnt; i++ {
		alpha := index2alpha(i, defaultAlphaCnt)
		i2 := alpha2index(alpha, defaultAlphaCnt)
		test.That(t, i, test.ShouldEqual, i2)
	}
}

func TestRevPTGs(t *testing.T) {
	cs := NewCirclePTG(0.3, 0.3)
	//~ cs := NewCSPTG(0.3, 0.3)
	_, err := ComputePTG(0, cs, -2000, 0.05)
	test.That(t, err, test.ShouldBeNil)
	//~ fmt.Println(traj)
	
}
