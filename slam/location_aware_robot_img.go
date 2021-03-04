package slam

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/utils"

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

var areaPointColor = color.NRGBA{255, 0, 0, 255}

func (lar *LocationAwareRobot) renderAreas(bounds image.Point, areas []*SquareArea) image.Image {
	// all areas are the same size
	_, scaleDown := areas[0].Size()
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(scaleDown) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(scaleDown) / lar.clientZoom))
	centerX := bounds.X / 2
	centerY := bounds.Y / 2

	dc := gg.NewContext(bounds.X, bounds.Y)

	// also serves as a font taking up 5% of space
	textScaleYStart := float64(bounds.Y) * .05
	rimage.DrawString(
		dc,
		fmt.Sprintf("zoom: %.02f", lar.clientZoom),
		image.Point{0, int(textScaleYStart)},
		rimage.Green,
		textScaleYStart/2)
	rimage.DrawString(
		dc,
		fmt.Sprintf("orientation: %.02f", lar.orientation()),
		image.Point{0, int(textScaleYStart * 1.5)},
		rimage.Green,
		textScaleYStart/2)

	basePosX, basePosY := lar.basePos()
	minX := basePosX - bounds.X/2
	maxX := basePosX + bounds.X/2
	minY := basePosY - bounds.Y/2
	maxY := basePosY + bounds.Y/2

	viewTranslateP := image.Point{-basePosX + centerX, -basePosY + centerY}
	relBaseRect := lar.baseRect().Add(viewTranslateP)

	rimage.DrawRectangleEmpty(dc, relBaseRect, color.NRGBA{0, 0, 255, 255}, 1)

	for _, orientation := range []float64{0, 90, 180, 270} {
		calcP, err := lar.calculateMove(orientation, defaultClientMoveAmount)
		if err == nil {
			moveRect := lar.moveRect(calcP.X, calcP.Y, orientation)
			moveRect = moveRect.Add(viewTranslateP)
			var c color.Color
			switch orientation {
			case 0:
				c = color.NRGBA{29, 131, 72, 255}
			case 90:
				c = color.NRGBA{23, 165, 137, 255}
			case 180:
				c = color.NRGBA{218, 247, 166, 255}
			case 270:
				c = color.NRGBA{255, 195, 0, 255}
			}
			rimage.DrawRectangleEmpty(dc, moveRect, c, 1)
		}
	}

	distance := 15.0
	x, y := utils.RayToUpwardCWCartesian(lar.orientation(), distance)
	relX := float64(centerX) + x
	relY := float64(centerY) - y // Y is decreasing in an image

	dc.DrawLine(float64(centerX), float64(centerY), relX, relY)
	dc.SetColor(color.NRGBA{0, 255, 0, 255})
	dc.SetLineWidth(3)
	dc.Stroke()

	// TODO(erd): any way to get a submatrix? may need to segment each one
	// if this starts going slower. fast as long as there are not many points
	for _, area := range areas {
		area.Mutate(func(area MutableArea) {
			area.Iterate(func(x, y, _ int) bool {
				if x < minX || x > maxX || y < minY || y > maxY {
					return true
				}
				distX := basePosX - x
				distY := basePosY - y
				relX := centerX - distX
				relY := centerY + distY // Y is decreasing in an image

				dc.SetColor(areaPointColor)
				dc.SetPixel(relX, relY)
				return true
			})
		})
	}

	return dc.Image()
}

func (lar *LocationAwareRobot) renderStoredView() (image.Image, error) {
	_, bounds, areas := lar.areasToView()
	return lar.renderAreas(bounds, areas), nil
}

func (lar *LocationAwareRobot) renderLiveView() (image.Image, error) {
	devices, bounds, areas := lar.areasToView()
	blankArea := areas[0].BlankCopy()

	if err := lar.scanAndStore(devices, blankArea); err != nil {
		return nil, err
	}

	return lar.renderAreas(bounds, []*SquareArea{blankArea}), nil
}
