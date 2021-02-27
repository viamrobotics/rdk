package kinematics

//~ import (
//~ "testing"

//~ "github.com/edaniels/test"
//~ "go.viam.com/robotcore/arm"
//~ "go.viam.com/robotcore/testutils"
//~ )

//~ func TestCreateIKSolver(t *testing.T) {
//~ m, err := ParseJSONFile(testutils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
//~ test.That(t, err, test.ShouldBeNil)
//~ ik := CreateNloptIKSolver(m)

//~ pos := arm.Position{0, -368, 355, 0, 0, 10}
//~ transform := kinmath.NewTransformFromRotation(pos.Rx, pos.Ry, pos.Rz)
//~ transform.SetX(pos.X)
//~ transform.SetY(pos.Y)
//~ transform.SetZ(pos.Z)

//~ k.ik.AddGoal(transform, k.effectorID)

//~ ik.Solve()
//~ }
