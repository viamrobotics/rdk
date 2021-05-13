package slam

import pb "go.viam.com/core/proto/slam/v1"

// DirectionFromXY returns a direction that originates from a x,y coordinate
// chosen. It determines this based on which quadrant the coordinate falls into:
// top left => up
// top right => down
// bottom left => left
// bottom right => right
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
