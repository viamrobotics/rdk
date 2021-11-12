package motionplan

import (
	"fmt"

	//~ "runtime"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"
)



var wbY = -480.

func TestWhiteboard(t *testing.T){
	oldGoal := spatial.NewPoseFromArmPos(&pb.ArmPosition{
		X:  480,
		Y:  wbY,
		Z:  600,
		OY: -1,
	})
	goal := spatial.NewPoseFromArmPos(&pb.ArmPosition{X:  440,Y:  wbY,Z:  500,OY: -1,})
	validOV := &spatial.OrientationVector{OX:0, OY:-1, OZ:0}
	validFunc, _ := NewLineConstraintAndGradient(oldGoal.Point(), goal.Point(), validOV)
	
	
	//~ p1 := spatial.NewPoseFromArmPos(&pb.ArmPosition{X:479.8361361711587,
						//~ Y:-480.08581873824187,
						//~ Z:600.2562446722525,
						//~ OX:-0.00025139303175381045,
						//~ OY:-0.9999998419942064,
						//~ OZ:0.0005028052361426005,
						//~ Theta:-0.044449533434648386})
	p2 := spatial.NewPoseFromArmPos(&pb.ArmPosition{X:479.8779170592054,
						Y:-480.00004548561094,
						Z:599.6951753254116,
						OX:-0.0023524376220581955,
						OY:-0.9996054198800094,
						OZ:-0.027990544541756812,
						Theta:2.027966162594598})
	
	fmt.Println(validFunc(ConstraintInput{startPos:p2, endPos:p2}))
}

