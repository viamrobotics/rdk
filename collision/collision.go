package collision

import (
	frame "go.viam.com/core/referenceframe"
)

func SelfCollision(model *frame.Model, input []frame.Input) (bool, error) {
	poses, err := model.VerboseTransform(input)
	if err != nil {
		return false, err
	}

	// update the pose of each valid volume in the model
	for joint, pose := range poses {
		if vol, ok := model.Volumes[joint]; ok {
			err := vol.UpdatePose(pose)
			if err != nil {
				return false, err
			}
		}
	}

	return true, nil
}
