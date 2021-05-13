package slam

import pb "go.viam.com/core/proto/slam/v1"

func DirectionFromXY(x, y, viewWidth, viewHeight int) pb.Direction {
	centerX := viewWidth / 2
	centerY := viewHeight / 2

	var dir pb.Direction
	if x < centerX {
		if y < centerY {
			dir = pb.Direction_DIRECTION_UP
		} else {
			dir = pb.Direction_DIRECTION_LEFT
		}
	} else {
		if y < centerY {
			dir = pb.Direction_DIRECTION_DOWN
		} else {
			dir = pb.Direction_DIRECTION_RIGHT
		}
	}
	return dir
}
