package motionplan

import (
	"fmt"

	//~ "runtime"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	spatial "go.viam.com/core/spatialmath"
)



//~ NOTE TO TOMORROW PETER:
//~ // I think that in constrainNear, sometimes the *seed* is not on the constraint (somehow), so it doesn't matter if the
//~ // target is. See this solved grad descent output, check points by hand?:

//~ p, o 5.509348801559717e-05 0
//~ p, o 5.452974012760906e-05 0
//~ p, o 5.503837831130655e-05 0
//~ p, o 5.4397730209688046e-05 0
//~ [{-0.034821059815564076} {0.33774133871029827} {-0.6328324286910566} {1.6840138488914407} {-1.239074561425037} {1.1155255593591178} {1.5944449697966563}] <nil>
//~ steps: 1 from x:439.78171921054627 y:-480.19764343681425 z:500.1596711655664 o_x:-0.2301799416570215 o_y:-0.9550491352975102 o_z:-0.18681098368738305 theta:-3.6316318999936343 to x:440.0891939811837 y:-479.99994910864143 z:500.223036688579 o_x:-0.22956570072665847 o_y:-0.9552609708054449 o_z:-0.18648342206672774 theta:-3.6440331702592985
//~ p, o 0.32816259517985985 0
//~ whiteboard failed, off by 0.10769068887518057
//~ no ok path

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
	
	fmt.Println(validFunc(constraintInput{startPos:p2, endPos:p2}))
}

