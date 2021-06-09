package robotimpl

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
)

// CreateReferenceFrameLookup takes a robot and implements the FrameLookup api
func CreateReferenceFrameLookup(ctx context.Context, r robot.Robot) (referenceframe.FrameLookup, error) {
	ref := &robotFrameRef{r, map[string]referenceframe.Frame{}}

	cfg, err := r.Config(ctx)
	if err != nil {
		return nil, err
	}

	for _, c := range cfg.Components {
		if c.Name == "" {
			return nil, errors.New("all components need names")
		}

		if c.Type == config.ComponentTypeArm {
			ref.frames[c.Name] = &armFrame{r, c}
		} else {
			pos := &pb.ArmPosition{}
			pos.X = c.ParentTranslation.X
			pos.Y = c.ParentTranslation.Y
			pos.Z = c.ParentTranslation.Z
			pos.OX = c.ParentOrientation.X
			pos.OY = c.ParentOrientation.Y
			pos.OZ = c.ParentOrientation.Z
			pos.Theta = c.ParentOrientation.TH
			ref.frames[c.Name] = referenceframe.NewBasicFrame(c.Name, c.Parent, pos)
		}

	}

	return ref, nil
}

type robotFrameRef struct {
	robot  robot.Robot
	frames map[string]referenceframe.Frame
}

func (rr *robotFrameRef) FindFrame(name string) referenceframe.Frame {
	return rr.frames[name]
}

type armFrame struct {
	robot  robot.Robot
	config config.Component
}

func (af *armFrame) Name() string {
	return af.config.Name
}

func (af *armFrame) Parent() string {
	return af.config.Parent
}

func (af *armFrame) OffsetFromParent(ctx context.Context) (*pb.ArmPosition, error) {
	arm := af.robot.ArmByName(af.config.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm named %s", af.config.Name)
	}
	return arm.CurrentPosition(ctx)
}
