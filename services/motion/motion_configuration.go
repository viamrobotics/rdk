package motion

import (
	"math"

	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/protoutils"
)

const (
	DefaultAngularDegsPerSec = 20.
	DefaultLinearMPerSec     = 0.3
	DefaultObstaclePollingHz = 1.
	DefaultPlanDeviationM    = 2.6
	DefaultPositionPollingHz = 1.
)

func configurationFromProto(motionCfg *pb.MotionConfiguration) *MotionConfiguration {
	obstacleDetectors := []ObstacleDetectorName{}
	planDeviationM := DefaultPlanDeviationM
	positionPollingHz := DefaultPositionPollingHz
	obstaclePollingHz := DefaultObstaclePollingHz
	linearMPerSec := DefaultLinearMPerSec
	angularDegsPerSec := DefaultAngularDegsPerSec

	if motionCfg != nil {
		if motionCfg.ObstacleDetectors != nil {
			for _, obstacleDetectorPair := range motionCfg.GetObstacleDetectors() {
				obstacleDetectors = append(obstacleDetectors, ObstacleDetectorName{
					VisionServiceName: protoutils.ResourceNameFromProto(obstacleDetectorPair.VisionService),
					CameraName:        protoutils.ResourceNameFromProto(obstacleDetectorPair.Camera),
				})
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
		ObstacleDetectors:     obstacleDetectors,
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

	if len(motionCfg.ObstacleDetectors) > 0 {
		pbObstacleDetector := []*pb.ObstacleDetector{}
		for _, obstacleDetectorPair := range motionCfg.ObstacleDetectors {
			pbObstacleDetector = append(pbObstacleDetector, &pb.ObstacleDetector{
				VisionService: protoutils.ResourceNameToProto(obstacleDetectorPair.VisionServiceName),
				Camera:        protoutils.ResourceNameToProto(obstacleDetectorPair.CameraName),
			})
		}
		proto.ObstacleDetectors = pbObstacleDetector
	}
	return proto
}
