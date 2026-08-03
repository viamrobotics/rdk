// Package kinematics holds the kinematics models for the arm models that the fake and simulated
// arms emulate. Both components read from this one copy so their models -- and the collision
// geometries the 3D scene draws from them -- cannot drift apart.
package kinematics

import (
	_ "embed"
	"fmt"

	"go.viam.com/rdk/referenceframe"
)

// The arm models that can be emulated, as spelled in the `arm-model` config attribute.
const (
	// Fake is the generic kinematics used when no real arm is being emulated.
	Fake   = "fake"
	UR5e   = "ur5e"
	UR7e   = "ur7e"
	UR20   = "ur20"
	XArm6  = "xarm6"
	XArm7  = "xarm7"
	Lite6  = "lite6"
	Dofbot = "dofbot"
)

//go:embed fake.json
var fakeJSON []byte

//go:embed ur5e.json
var ur5eJSON []byte

//go:embed ur7e.json
var ur7eJSON []byte

//go:embed ur20.json
var ur20JSON []byte

//go:embed xarm6.json
var xarm6JSON []byte

//go:embed xarm7.json
var xarm7JSON []byte

//go:embed lite6.json
var lite6JSON []byte

//go:embed dofbot.json
var dofbotJSON []byte

var modelJSON = map[string][]byte{
	Fake:   fakeJSON,
	UR5e:   ur5eJSON,
	UR7e:   ur7eJSON,
	UR20:   ur20JSON,
	XArm6:  xarm6JSON,
	XArm7:  xarm7JSON,
	Lite6:  lite6JSON,
	Dofbot: dofbotJSON,
}

// ModelFromName returns the kinematics model for the given arm model, built under the given
// resource name.
func ModelFromName(armModel, name string) (referenceframe.Model, error) {
	kinematicsJSON, ok := modelJSON[armModel]
	if !ok {
		return nil, fmt.Errorf("arm cannot be created, unsupported arm-model: %s", armModel)
	}
	return referenceframe.UnmarshalModelJSON(kinematicsJSON, name)
}
