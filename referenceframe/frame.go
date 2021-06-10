// Package referenceframe defines the api and does the math of translating between reference frames
// Useful for if you have a camera, connected to a gripper, connected to an arm,
// and need to translate the camera reference frame to the arm reference frame,
// if you've found something in the camera, and want to move the gripper + arm to get it.
package referenceframe

import (
	"context"
	"errors"
	"fmt"

	"gonum.org/v1/gonum/num/dualquat"

	"go.viam.com/core/kinematics/kinmath"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"
)

// OffsetBy takes two offsets and computes the final position
func OffsetBy(a, b *pb.ArmPosition) *pb.ArmPosition {
	q1 := kinmath.NewQuatTransFromArmPos(a)
	q2 := kinmath.NewQuatTransFromArmPos(b)
	q3 := q1.Transformation(q2.Quat)
	final := &pb.ArmPosition{}
	cartQuat := dualquat.Mul(q3, dualquat.Conj(q3))
	final.X = cartQuat.Dual.Imag
	final.Y = cartQuat.Dual.Jmag
	final.Z = cartQuat.Dual.Kmag
	poseOV := kinmath.QuatToOV(q3.Real)
	final.Theta = utils.RadToDeg(poseOV.Theta)
	final.OX = poseOV.OX
	final.OY = poseOV.OY
	final.OZ = poseOV.OZ

	return final
}

// Frame represents a single reference frame, e.g. an arm
type Frame interface {
	Name() string
	Parent() string
	OffsetFromParent(ctx context.Context) (*pb.ArmPosition, error)
}

// FrameLookup is a way to find frames from some source
type FrameLookup interface {
	// FindFrame will find frame with name, or return nil if it can't find it
	FindFrame(name string) Frame
}

// FindTranslationChildToParent finds the path from one frame to other and computes the translation
func FindTranslationChildToParent(ctx context.Context, lookup FrameLookup, childName, parentName string) (*pb.ArmPosition, error) {
	origChild := childName
	seen := map[string]bool{}
	path := []string{}

	offsets := []*pb.ArmPosition{}
	for {
		if seen[childName] {
			return nil, errors.New("infinite loop in FindTranslationChildToParent")
		}
		seen[childName] = true
		path = append(path, childName)

		child := lookup.FindFrame(childName)
		if child == nil {
			return nil, fmt.Errorf("could not find frame: (%s) in lookup (%s) -> (%s) path: %v",
				childName, origChild, parentName, path)
		}

		myoffset, err := child.OffsetFromParent(ctx)
		if err != nil {
			return nil, err
		}

		if myoffset != nil {
			// if the offset is nil, we assume it has no impact
			offsets = append(offsets, myoffset)
		}

		if childName == parentName {
			break
		}

		childName = child.Parent()
	}

	offset := &pb.ArmPosition{}
	for i := 0; i < len(offsets); i++ {
		offset = OffsetBy(offset, offsets[len(offsets)-1-i])
	}
	return offset, nil
}

// ------

// NewBasicFrame creates a simple immutable frame
func NewBasicFrame(name, parent string, offset *pb.ArmPosition) Frame {
	return &basicFrame{name, parent, offset}
}

type basicFrame struct {
	name   string
	parent string
	offset *pb.ArmPosition
}

func (f *basicFrame) Name() string {
	return f.name
}

func (f *basicFrame) Parent() string {
	return f.parent
}

func (f *basicFrame) OffsetFromParent(ctx context.Context) (*pb.ArmPosition, error) {
	return f.offset, nil
}

// ------

type basicFrameMap map[string]Frame

func (m *basicFrameMap) FindFrame(name string) Frame {
	return (*m)[name]
}

func (m *basicFrameMap) add(f Frame) {
	(*m)[f.Name()] = f
}
