package motion

import (
	"math"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// MotionConfiguration specifies how to configure a call to the a motion service.
//
//nolint:revive
type MotionConfiguration struct {
	VisionServices        []resource.Name
	PositionPollingFreqHz float64
	ObstaclePollingFreqHz float64
	PlanDeviationMM       float64
	LinearMPerSec         float64
	AngularDegsPerSec     float64
}

func configurationFromProto(motionCfg *pb.MotionConfiguration) *MotionConfiguration {
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

func (motionCfg MotionConfiguration) toProto() *pb.MotionConfiguration {
	proto := &pb.MotionConfiguration{}
	if !math.IsNaN(motionCfg.LinearMPerSec) && motionCfg.LinearMPerSec != 0 {
		proto.LinearMPerSec = &motionCfg.LinearMPerSec
	}
	if !math.IsNaN(motionCfg.AngularDegsPerSec) && motionCfg.AngularDegsPerSec != 0 {
		proto.AngularDegsPerSec = &motionCfg.AngularDegsPerSec
	}
	if !math.IsNaN(motionCfg.ObstaclePollingFreqHz) && motionCfg.ObstaclePollingFreqHz > 0 {
		proto.ObstaclePollingFrequencyHz = &motionCfg.ObstaclePollingFreqHz
	}
	if !math.IsNaN(motionCfg.PositionPollingFreqHz) && motionCfg.PositionPollingFreqHz > 0 {
		proto.PositionPollingFrequencyHz = &motionCfg.PositionPollingFreqHz
	}
	if !math.IsNaN(motionCfg.PlanDeviationMM) && motionCfg.PlanDeviationMM >= 0 {
		planDeviationM := 1e-3 * motionCfg.PlanDeviationMM
		proto.PlanDeviationM = &planDeviationM
	}

	if len(motionCfg.VisionServices) > 0 {
		svcs := []*commonpb.ResourceName{}
		for _, name := range motionCfg.VisionServices {
			svcs = append(svcs, protoutils.ResourceNameToProto(name))
		}
		proto.VisionServices = svcs
	}
	return proto
}
