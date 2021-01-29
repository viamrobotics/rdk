package slam

import (
	"image"
	"image/color"
	"math"

	"github.com/echolabsinc/robotcore/utils"

	"gocv.io/x/gocv"
)

func (lar *LocationAwareRobot) NextMat() (gocv.Mat, error) {
	lar.update()

	// select device and sparse
	bounds, area, err := lar.areaToView()
	if err != nil {
		return gocv.Mat{}, err
	}

	_, scaleDown := area.Size()
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	out := gocv.NewMatWithSize(bounds.X, bounds.Y, gocv.MatTypeCV8UC3)

	var drawLine bool
	// drawLine = true

	basePosX, basePosY := lar.basePos()
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	area.Mutate(func(area MutableArea) {
		area.DoNonZero(func(x, y int, _ float64) {
			if x < minX || x > maxX || y < minY || y > maxY {
				return
			}
			distX := basePosX - x
			distY := basePosY - y
			relX := centerX - distX
			relY := centerY - distY

			p := image.Point{relX, relY}
			if drawLine {
				gocv.Line(&out, image.Point{centerX, centerY}, p, color.RGBA{R: 255}, 1)
			} else {
				gocv.Circle(&out, p, 4, color.RGBA{R: 255}, 1)
			}
		})
	})

	for i, orientation := range lar.orientations {
		if math.IsInf(orientation, 1) {
			continue
		}
		distance := 20.0
		// Remember, our view is from x,y=0,0 at top left of matrix
		// 0°   -  (0,-1) // Up
		// 90°  -  (1, 0) // Right
		// 180° -  (0, 1) // Down
		// 270° -  (-1,0) // Left
		orientationRads := utils.DegToRad(orientation)
		x := distance * math.Sin(orientationRads)
		y := distance * -math.Cos(orientationRads)
		relX := centerX + int(x)
		relY := centerY + int(y)
		p := image.Point{relX, relY}

		if i == 0 {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{G: 255}, 5)
		} else {
			gocv.ArrowedLine(&out, image.Point{centerX, centerY}, p, color.RGBA{B: 255}, 5)
		}
	}

	return out, nil
}
