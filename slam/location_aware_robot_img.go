package slam

import (
	"context"
	"image"
	"image/color"
	"math"

	"github.com/viamrobotics/robotcore/utils"

	"github.com/fogleman/gg"
)

func (lar *LocationAwareRobot) Next(ctx context.Context) (image.Image, error) {
	// select device and sparse
	bounds, area, err := lar.areaToView()
	if err != nil {
		return nil, err
	}

	_, scaleDown := area.Size()
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	dc := gg.NewContext(bounds.X, bounds.Y)

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

			dc.DrawPoint(float64(relX), float64(relY), 1)
			dc.SetColor(color.RGBA{255, 0, 0, 255})
			dc.Fill()
		})
	})

	for _, orientation := range []int{0, 90, 180, 270} {
		distance := 20.0
		// Remember, our view is from x,y=0,0 at top left of matrix
		// 0°   -  (0,-1) // Up
		// 90°  -  (1, 0) // Right
		// 180° -  (0, 1) // Down
		// 270° -  (-1,0) // Left
		orientationRads := utils.DegToRad(float64(orientation))
		x := distance * math.Sin(orientationRads)
		y := distance * -math.Cos(orientationRads)
		relX := float64(centerX) + x
		relY := float64(centerY) + y

		dc.DrawPoint(relX, relY, 1)
		dc.SetColor(color.RGBA{0, 255, 0, 255})
		dc.Fill()
	}

	for i, orientation := range lar.orientations {
		if math.IsInf(orientation, 1) {
			continue
		}
		distance := 15.0
		// Remember, our view is from x,y=0,0 at top left of matrix
		// 0°   -  (0,-1) // Up
		// 90°  -  (1, 0) // Right
		// 180° -  (0, 1) // Down
		// 270° -  (-1,0) // Left
		orientationRads := utils.DegToRad(orientation)
		x := distance * math.Sin(orientationRads)
		y := distance * -math.Cos(orientationRads)
		relX := float64(centerX) + x
		relY := float64(centerY) + y

		dc.DrawLine(float64(centerX), float64(centerY), relX, relY)
		if i == 0 {
			dc.SetColor(color.RGBA{0, 255, 0, 255})
		} else {
			dc.SetColor(color.RGBA{0, 0, 255, 255})
		}
		dc.SetLineWidth(3)
		dc.Stroke()
	}

	return dc.Image(), nil
}
