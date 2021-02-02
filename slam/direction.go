package slam

type Direction string

const (
	DirectionUp    = "up"
	DirectionRight = "right"
	DirectionDown  = "down"
	DirectionLeft  = "left"
)

func DirectionFromXY(x, y, viewWidth, viewHeight int) Direction {
	centerX := viewWidth / 2
	centerY := viewHeight / 2

	var rotateTo Direction
	if x < centerX {
		if y < centerY {
			rotateTo = DirectionUp
		} else {
			rotateTo = DirectionLeft
		}
	} else {
		if y < centerY {
			rotateTo = DirectionDown
		} else {
			rotateTo = DirectionRight
		}
	}
	return rotateTo
}
