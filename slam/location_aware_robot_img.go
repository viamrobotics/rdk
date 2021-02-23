package slam

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/viamrobotics/robotcore/utils"

	"github.com/fogleman/gg"
)

func (lar *LocationAwareRobot) Next(ctx context.Context) (image.Image, error) {
	switch lar.clientLidarViewMode {
	case clientLidarViewModeStored:
		return lar.renderStoredView()
	case clientLidarViewModeLive:
		return lar.renderLiveView()
	default:
		return nil, fmt.Errorf("unknown view mode %q", lar.clientLidarViewMode)
	}
}

func (lar *LocationAwareRobot) renderAreas(bounds image.Point, areas []*SquareArea, orientations []float64) (image.Image, error) {
	// all areas are the same size
	_, scaleDown := areas[0].Size()
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	dc := gg.NewContext(bounds.X, bounds.Y)

	// also serves as a font taking up 5% of space
	textScaleYStart := float64(bounds.Y) * .05
	utils.DrawString(
		dc,
		fmt.Sprintf("zoom: %.02f", lar.clientZoom),
		image.Point{0, int(textScaleYStart)},
		utils.Green.C,
		textScaleYStart/2)
	utils.DrawString(
		dc,
		fmt.Sprintf("orientation: %.02f", lar.orientation()),
		image.Point{0, int(textScaleYStart * 1.5)},
		utils.Green.C,
		textScaleYStart/2)

	basePosX, basePosY := lar.basePos()
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	viewTranslateP := image.Point{-basePosX + centerX, -basePosY + centerY}
	relBaseRect := lar.baseRect().Add(viewTranslateP)

	utils.DrawRectangleEmpty(dc, relBaseRect, color.RGBA{0, 0, 255, 255}, 1)

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	for _, area := range areas {
		area.Mutate(func(area MutableArea) {
			area.Iterate(func(x, y int, _ float64) bool {
				if x < minX || x > maxX || y < minY || y > maxY {
					return true
				}
				distX := basePosX - x
				distY := basePosY - y
				relX := centerX - distX
				relY := centerY - distY

				dc.SetColor(color.RGBA{255, 0, 0, 255})
				dc.SetPixel(relX, relY)
				return true
			})
		})
	}

	for _, orientation := range []float64{0, 90, 180, 270} {
		calcP, _, err := lar.calculateMove(orientation, defaultClientMoveAmount)
		if err == nil {
			moveRect := lar.moveRect(calcP.X, calcP.Y, orientation)
			moveRect = moveRect.Add(viewTranslateP)
			var c color.Color
			switch orientation {
			case 0:
				c = color.RGBA{29, 131, 72, 255}
			case 90:
				c = color.RGBA{23, 165, 137, 255}
			case 180:
				c = color.RGBA{218, 247, 166, 255}
			case 270:
				c = color.RGBA{255, 195, 0, 255}
			}
			utils.DrawRectangleEmpty(dc, moveRect, c, 1)
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
		relX := float64(centerX) + x
		relY := float64(centerY) + y

		dc.SetColor(color.RGBA{0, 255, 0, 255})
		dc.SetPixel(int(relX), int(relY))
	}

	for i, orientation := range orientations {
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

func (lar *LocationAwareRobot) renderStoredView() (image.Image, error) {
	_, bounds, areas, err := lar.areasToView()
	if err != nil {
		return nil, err
	}

	return lar.renderAreas(bounds, areas, lar.orientations)
}

func (lar *LocationAwareRobot) renderLiveView() (image.Image, error) {
	devices, bounds, areas, err := lar.areasToView()
	if err != nil {
		return nil, err
	}

	meters, scaleTo := areas[0].Size()
	blankArea := NewSquareArea(meters, scaleTo)

	orientations, err := lar.scanAndStore(devices, blankArea)
	if err != nil {
		return nil, err
	}

	return lar.renderAreas(bounds, []*SquareArea{blankArea}, orientations)
}
