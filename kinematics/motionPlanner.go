package kinematics

type MotionPlanner interface {
	InverseKinematics
	cfg MotionPlannerConfig
}
