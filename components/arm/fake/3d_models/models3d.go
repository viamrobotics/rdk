// Package models3d holds the embedded visual meshes of the arm models the fake and simulated arms
// can stand in for, and attaches them to a model as visual geometries keyed by link.
package models3d

import (
	_ "embed"
	"fmt"
	"maps"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

//go:embed ur5e/base_link.glb
var ur5eBaseLinkGLB []byte

//go:embed ur5e/ee_link.glb
var ur5eEELinkGLB []byte

//go:embed ur5e/forearm_link.glb
var ur5eForearmLinkGLB []byte

//go:embed ur5e/upper_arm_link.glb
var ur5eUpperArmLinkGLB []byte

//go:embed ur5e/wrist_1_link.glb
var ur5eWrist1LinkGLB []byte

//go:embed ur5e/wrist_2_link.glb
var ur5eWrist2LinkGLB []byte

//go:embed ur5e/shoulder_link.glb
var ur5eShoulderLinkGLB []byte

//go:embed ur20/base_link.glb
var ur20BaseLinkGLB []byte

//go:embed ur20/shoulder_link.glb
var ur20ShoulderLinkGLB []byte

//go:embed ur20/upper_arm_link.glb
var ur20UpperArmLinkGLB []byte

//go:embed ur20/forearm_link.glb
var ur20ForearmLinkGLB []byte

//go:embed ur20/wrist_1_link.glb
var ur20Wrist1LinkGLB []byte

//go:embed ur20/wrist_2_link.glb
var ur20Wrist2LinkGLB []byte

//go:embed ur20/wrist_3_link.glb
var ur20Wrist3LinkGLB []byte

//go:embed xarm6/base.glb
var xarm6BaseGLB []byte

//go:embed xarm6/base_top.glb
var xarm6BaseTopGLB []byte

//go:embed xarm6/upper_arm.glb
var xarm6UpperArmGLB []byte

//go:embed xarm6/upper_forearm.glb
var xarm6UpperForearmGLB []byte

//go:embed xarm6/lower_forearm.glb
var xarm6LowerForearmGLB []byte

//go:embed xarm6/wrist_link.glb
var xarm6WristLinkGLB []byte

//go:embed xarm6/gripper_mount.glb
var xarm6GripperMountGLB []byte

//go:embed lite6/base.glb
var lite6BaseGLB []byte

//go:embed lite6/base_top.glb
var lite6BaseTopGLB []byte

//go:embed lite6/upper_arm.glb
var lite6UpperArmGLB []byte

//go:embed lite6/upper_forearm.glb
var lite6UpperForearmGLB []byte

//go:embed lite6/lower_forearm.glb
var lite6LowerForearmGLB []byte

//go:embed lite6/wrist_link.glb
var lite6WristLinkGLB []byte

//go:embed lite6/gripper_mount.glb
var lite6GripperMountGLB []byte

// ContentType is the spelling the viewer reads a GLB by today. The file loader takes the extension
// spelling, glb, as well, and the viewer moves to it with RSDK-14759.
const ContentType = "model/gltf-binary"

// visuals maps an arm model to the GLB of each link that has one, keyed by the link id in the
// embedded kinematics file of the same name.
var visuals = map[string]map[string][]byte{
	"ur5e": {
		"base_link":      ur5eBaseLinkGLB,
		"shoulder_link":  ur5eShoulderLinkGLB,
		"upper_arm_link": ur5eUpperArmLinkGLB,
		"forearm_link":   ur5eForearmLinkGLB,
		"wrist_1_link":   ur5eWrist1LinkGLB,
		"wrist_2_link":   ur5eWrist2LinkGLB,
		"ee_link":        ur5eEELinkGLB,
	},
	"ur20": {
		"base_link":      ur20BaseLinkGLB,
		"shoulder_link":  ur20ShoulderLinkGLB,
		"upper_arm_link": ur20UpperArmLinkGLB,
		"forearm_link":   ur20ForearmLinkGLB,
		"wrist_1_link":   ur20Wrist1LinkGLB,
		"wrist_2_link":   ur20Wrist2LinkGLB,
		"wrist_3_link":   ur20Wrist3LinkGLB,
	},
	"xarm6": {
		"base":          xarm6BaseGLB,
		"base_top":      xarm6BaseTopGLB,
		"upper_arm":     xarm6UpperArmGLB,
		"upper_forearm": xarm6UpperForearmGLB,
		"lower_forearm": xarm6LowerForearmGLB,
		"wrist_link":    xarm6WristLinkGLB,
		"gripper_mount": xarm6GripperMountGLB,
	},
	"lite6": {
		"base":          lite6BaseGLB,
		"base_top":      lite6BaseTopGLB,
		"upper_arm":     lite6UpperArmGLB,
		"upper_forearm": lite6UpperForearmGLB,
		"lower_forearm": lite6LowerForearmGLB,
		"wrist_link":    lite6WristLinkGLB,
		"gripper_mount": lite6GripperMountGLB,
	},
}

// Visuals returns the GLB of each link of an arm model that has one, keyed by link id, or nil for
// a model that ships none. The bytes are the embedded files and are not to be written to.
func Visuals(armModel string) map[string][]byte {
	if len(visuals[armModel]) == 0 {
		return nil
	}
	return maps.Clone(visuals[armModel])
}

// AttachVisuals sets each embedded GLB of an arm model on the matching link of model as a visual
// geometry, so the typed model carries them and Get3DModels can be served from it. An arm model
// that ships none leaves model as it is. A GLB named for a link the model does not have is an
// error, since the kinematics file and the meshes have drifted apart.
func AttachVisuals(model referenceframe.Model, armModel string) error {
	if len(visuals[armModel]) == 0 {
		return nil
	}
	sm, ok := model.(*referenceframe.SimpleModel)
	if !ok {
		return fmt.Errorf("cannot attach visuals to a %T", model)
	}
	for link, glb := range visuals[armModel] {
		gc := spatialmath.GeometryConfig{
			Type:            spatialmath.MeshType,
			Label:           link,
			MeshData:        glb,
			MeshContentType: ContentType,
			MeshFilePath:    armModel + "/" + link + ".glb",
		}
		if err := sm.SetVisualGeometries(link, []spatialmath.GeometryConfig{gc}); err != nil {
			return fmt.Errorf("arm model %q: %w", armModel, err)
		}
	}
	return nil
}
