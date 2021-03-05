package slam

type Direction string

const (
	DirectionUp    = Direction("up")
	DirectionRight = Direction("right")
	DirectionDown  = Direction("down")
	DirectionLeft  = Direction("left")
)

func DirectionFromXY(x, y, viewWidth, viewHeight int) Direction {
	centerX := viewWidth / 2
	centerY := viewHeight / 2

	var dir Direction
	if x < centerX {
		if y < centerY {
			dir = DirectionUp
		} else {
			dir = DirectionLeft
		}
	} else {
		if y < centerY {
			dir = DirectionDown
		} else {
			dir = DirectionRight
		}
	}
	return dir
}
