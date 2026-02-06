// Package models3d contains embedded 3D model files and related mappings for fake arm models.
package models3d

import (
	_ "embed"

	commonpb "go.viam.com/api/common/v1"
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

//go:embed so101/base.glb
var so101BaseGLB []byte

//go:embed so101/shoulder.glb
var so101ShoulderGLB []byte

//go:embed so101/upper_arm.glb
var so101UpperArmGLB []byte

//go:embed so101/lower_arm.glb
var so101LowerArmGLB []byte

//go:embed so101/wrist.glb
var so101WristGLB []byte

// ArmTo3DModelParts maps arm model names to their list of 3D model part names.
var ArmTo3DModelParts = map[string][]string{
	"ur5e": {
		"ee_link",
		"forearm_link",
		"upper_arm_link",
		"wrist_1_link",
		"wrist_2_link",
		"base_link",
		"shoulder_link",
	},
	"ur20": {
		"base_link",
		"shoulder_link",
		"upper_arm_link",
		"forearm_link",
		"wrist_1_link",
		"wrist_2_link",
		"wrist_3_link",
	},
	"xarm6": {
		"base",
		"base_top",
		"upper_arm",
		"upper_forearm",
		"lower_forearm",
		"wrist_link",
		"gripper_mount",
	},
	"lite6": {
		"base",
		"base_top",
		"upper_arm",
		"upper_forearm",
		"lower_forearm",
		"wrist_link",
		"gripper_mount",
	},
	"so101": {
		"base",
		"shoulder",
		"upper_arm",
		"lower_arm",
		"wrist",
	},
}

// ThreeDMeshFromName returns the 3D mesh for a given arm model and part name.
func ThreeDMeshFromName(model, name string) commonpb.Mesh {
	switch model {
	case "ur5e":
		switch name {
		case "base_link":
			return commonpb.Mesh{
				Mesh:        ur5eBaseLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "ee_link":
			return commonpb.Mesh{
				Mesh:        ur5eEELinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "shoulder_link":
			return commonpb.Mesh{
				Mesh:        ur5eShoulderLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "forearm_link":
			return commonpb.Mesh{
				Mesh:        ur5eForearmLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_arm_link":
			return commonpb.Mesh{
				Mesh:        ur5eUpperArmLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_1_link":
			return commonpb.Mesh{
				Mesh:        ur5eWrist1LinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_2_link":
			return commonpb.Mesh{
				Mesh:        ur5eWrist2LinkGLB,
				ContentType: "model/gltf-binary",
			}
		}
	case "ur20":
		switch name {
		case "base_link":
			return commonpb.Mesh{
				Mesh:        ur20BaseLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "shoulder_link":
			return commonpb.Mesh{
				Mesh:        ur20ShoulderLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_arm_link":
			return commonpb.Mesh{
				Mesh:        ur20UpperArmLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "forearm_link":
			return commonpb.Mesh{
				Mesh:        ur20ForearmLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_1_link":
			return commonpb.Mesh{
				Mesh:        ur20Wrist1LinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_2_link":
			return commonpb.Mesh{
				Mesh:        ur20Wrist2LinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_3_link":
			return commonpb.Mesh{
				Mesh:        ur20Wrist3LinkGLB,
				ContentType: "model/gltf-binary",
			}
		}
	case "xarm6":
		switch name {
		case "base":
			return commonpb.Mesh{
				Mesh:        xarm6BaseGLB,
				ContentType: "model/gltf-binary",
			}
		case "base_top":
			return commonpb.Mesh{
				Mesh:        xarm6BaseTopGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_arm":
			return commonpb.Mesh{
				Mesh:        xarm6UpperArmGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_forearm":
			return commonpb.Mesh{
				Mesh:        xarm6UpperForearmGLB,
				ContentType: "model/gltf-binary",
			}
		case "lower_forearm":
			return commonpb.Mesh{
				Mesh:        xarm6LowerForearmGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_link":
			return commonpb.Mesh{
				Mesh:        xarm6WristLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "gripper_mount":
			return commonpb.Mesh{
				Mesh:        xarm6GripperMountGLB,
				ContentType: "model/gltf-binary",
			}
		}
	case "lite6":
		switch name {
		case "base":
			return commonpb.Mesh{
				Mesh:        lite6BaseGLB,
				ContentType: "model/gltf-binary",
			}
		case "base_top":
			return commonpb.Mesh{
				Mesh:        lite6BaseTopGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_arm":
			return commonpb.Mesh{
				Mesh:        lite6UpperArmGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_forearm":
			return commonpb.Mesh{
				Mesh:        lite6UpperForearmGLB,
				ContentType: "model/gltf-binary",
			}
		case "lower_forearm":
			return commonpb.Mesh{
				Mesh:        lite6LowerForearmGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist_link":
			return commonpb.Mesh{
				Mesh:        lite6WristLinkGLB,
				ContentType: "model/gltf-binary",
			}
		case "gripper_mount":
			return commonpb.Mesh{
				Mesh:        lite6GripperMountGLB,
				ContentType: "model/gltf-binary",
			}
		}
	case "so101":
		switch name {
		case "base":
			return commonpb.Mesh{
				Mesh:        so101BaseGLB,
				ContentType: "model/gltf-binary",
			}

		case "shoulder":
			return commonpb.Mesh{
				Mesh:        so101ShoulderGLB,
				ContentType: "model/gltf-binary",
			}
		case "upper_arm":
			return commonpb.Mesh{
				Mesh:        so101UpperArmGLB,
				ContentType: "model/gltf-binary",
			}
		case "lower_arm":
			return commonpb.Mesh{
				Mesh:        so101LowerArmGLB,
				ContentType: "model/gltf-binary",
			}
		case "wrist":
			return commonpb.Mesh{
				Mesh:        so101WristGLB,
				ContentType: "model/gltf-binary",
			}
		}
	}
	return commonpb.Mesh{}
}
