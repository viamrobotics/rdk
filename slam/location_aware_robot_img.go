package slam

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	pb "go.viam.com/robotcore/proto/slam/v1"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/utils"

	"github.com/fogleman/gg"
)

func (lar *LocationAwareRobot) Next(ctx context.Context) (image.Image, func(), error) {
	switch lar.clientLidarViewMode {
	case pb.LidarViewMode_LIDAR_VIEW_MODE_STORED:
		return lar.renderStoredView()
	case pb.LidarViewMode_LIDAR_VIEW_MODE_LIVE:
		return lar.renderLiveView(ctx)
	default:
		return nil, nil, fmt.Errorf("unknown view mode %q", lar.clientLidarViewMode)
	}
}

var areaPointColor = color.NRGBA{255, 0, 0, 255}

func (lar *LocationAwareRobot) renderAreas(bounds image.Point, areas []*SquareArea) (image.Image, error) {
	// all areas are the same size
	bounds.X = int(math.Ceil(float64(bounds.X) * float64(lar.unitsPerMeter) / lar.clientZoom))
	bounds.Y = int(math.Ceil(float64(bounds.Y) * float64(lar.unitsPerMeter) / lar.clientZoom))
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
		calcP, err := lar.calculateMove(orientation, defaultClientMoveAmountMillis)
		if err == nil {
			moveRect, err := lar.moveRect(calcP.X, calcP.Y, orientation)
			if err != nil {
				return nil, err
			}
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

	distance := 30.0
	x, y := utils.RayToUpwardCWCartesian(lar.orientation(), distance)
	relX := float64(centerX) + x
	relY := float64(centerY) - y // Y is decreasing in an image

	dc.DrawLine(float64(centerX), float64(centerY), relX, relY)
	dc.SetColor(color.NRGBA{0, 255, 0, 255})
	dc.SetLineWidth(3)
	dc.Stroke()

	// If this starts going slower, will need a more optimal way of asking
	// for a sub-area; it's fast as long as there are not many points
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

	return dc.Image(), nil
}

func (lar *LocationAwareRobot) renderStoredView() (image.Image, func(), error) {
	_, bounds, areas := lar.areasToView()
	img, err := lar.renderAreas(bounds, areas)
	return img, func() {}, err
}

func (lar *LocationAwareRobot) renderLiveView(ctx context.Context) (image.Image, func(), error) {
	devices, bounds, areas := lar.areasToView()
	blankArea, err := areas[0].BlankCopy(lar.logger)
	if err != nil {
		return nil, nil, err
	}

	if err := lar.scanAndStore(ctx, devices, blankArea); err != nil {
		return nil, nil, err
	}

	img, err := lar.renderAreas(bounds, []*SquareArea{blankArea})
	return img, func() {}, err
}
