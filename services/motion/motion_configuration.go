package motion

import (
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// MotionConfiguration specifies how to configure a call to the a motion service
type MotionConfiguration struct {
	VisionServices        []resource.Name
	PositionPollingFreqHz float64
	ObstaclePollingFreqHz float64
	PlanDeviationMM       float64
	LinearMPerSec         float64
	AngularDegsPerSec     float64
}

func motionConfigurationFromProto(motionCfg *pb.MotionConfiguration) *MotionConfiguration {
	visionSvc := []resource.Name{}
	planDeviationM := 0.
	positionPollingHz := 0.
	obstaclePollingHz := 0.
	linearMPerSec := 0.
	angularDegsPerSec := 0.

	if motionCfg != nil {
		if motionCfg.VisionServices != nil {
			for _, name := range motionCfg.GetVisionServices() {
				visionSvc = append(visionSvc, protoutils.ResourceNameFromProto(name))
			}
		}
		if motionCfg.PositionPollingFrequencyHz != nil {
			positionPollingHz = motionCfg.GetPositionPollingFrequencyHz()
		}
		if motionCfg.ObstaclePollingFrequencyHz != nil {
			obstaclePollingHz = motionCfg.GetObstaclePollingFrequencyHz()
		}
		if motionCfg.PlanDeviationM != nil {
			planDeviationM = motionCfg.GetPlanDeviationM()
		}
		if motionCfg.LinearMPerSec != nil {
			linearMPerSec = motionCfg.GetLinearMPerSec()
		}
		if motionCfg.AngularDegsPerSec != nil {
			angularDegsPerSec = motionCfg.GetAngularDegsPerSec()
		}
	}

	return &MotionConfiguration{
		VisionServices:        visionSvc,
		PositionPollingFreqHz: positionPollingHz,
		ObstaclePollingFreqHz: obstaclePollingHz,
		PlanDeviationMM:       1e3 * planDeviationM,
		LinearMPerSec:         linearMPerSec,
		AngularDegsPerSec:     angularDegsPerSec,
	}
}
