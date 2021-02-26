package vision

import (
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/utils"
)

type PointCloud struct {
	Depth *DepthMap
	Color Image
}

func (pc *PointCloud) Width() int {
	return pc.Color.Width()
}

func (pc *PointCloud) Height() int {
	return pc.Color.Height()
}

func (pc *PointCloud) Warp(src, dst []image.Point, newSize image.Point) PointCloud {
	m2 := utils.GetPerspectiveTransform(src, dst)

	img := utils.WarpImage(pc.Color.Image(), m2, newSize)

	var warpedDepth *DepthMap
	if pc.Depth.Width() > 0 {
		dm2 := pc.Depth.Warp(m2, newSize)
		warpedDepth = &dm2
	}

	return PointCloud{warpedDepth, NewImage(img)}
}

func (pc *PointCloud) CropToDepthData() (PointCloud, error) {
	var minY, minX, maxY, maxX int

	for minY = 0; minY < pc.Height(); minY++ {
		found := false
		for x := 0; x < pc.Width(); x++ {
			if pc.Depth.GetDepth(x, minY) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	for maxY = pc.Height() - 1; maxY >= 0; maxY-- {
		found := false
		for x := 0; x < pc.Width(); x++ {
			if pc.Depth.GetDepth(x, maxY) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if maxY <= minY {
		return PointCloud{}, fmt.Errorf("invalid depth data: %v %v", minY, maxY)
	}

	for minX = 0; minX < pc.Width(); minX++ {
		found := false
		for y := minY; y < maxY; y++ {
			if pc.Depth.GetDepth(minX, y) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	for maxX = pc.Width() - 1; minX >= 0; maxX-- {
		found := false
		for y := minY; y < maxY; y++ {
			if pc.Depth.GetDepth(maxX, y) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	height := maxY - minY
	width := maxX - minX

	return pc.Warp(
		[]image.Point{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}},
		[]image.Point{{0, 0}, {width, 0}, {width, height}, {0, height}},
		image.Point{width, height},
	), nil
}

func (pc *PointCloud) Close() {

}

func NewPointCloud(colorFN, depthFN string) (*PointCloud, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, err
	}

	dm, err := ParseDepthMap(depthFN)
	if err != nil {
		return nil, err
	}

	if img.Width() != dm.Width() || img.Height() != dm.Height() {
		return nil, fmt.Errorf("color and depth size doesn't match %d,%d vs %d,%d",
			img.Width(), img.Height(), dm.Width(), dm.Height())
	}

	return &PointCloud{dm, img}, nil
}

func _colorToInt(c color.RGBA) int {
	x := 0

	x = x | (int(c.R) << 16)
	x = x | (int(c.G) << 8)
	x = x | (int(c.B) << 0)
	//x = x | ( int(c.A) << 0 )

	return x
}

func _scale(x, max int) float32 {
	res := float32(x) / float32(max)
	return res - .5
}
func (pc *PointCloud) ToPCD(out io.Writer) error {
	if pc.Depth.Width() != pc.Color.Width() ||
		pc.Depth.Height() != pc.Color.Height() {
		return fmt.Errorf("depth map and color dimensions don't match %d,%d -> %d,%d",
			pc.Depth.Width(), pc.Depth.Height(), pc.Color.Width(), pc.Color.Height())
	}

	_, err := fmt.Fprintf(out, "VERSION .7\n"+
		"FIELDS x y z rgb\n"+
		"SIZE 4 4 4 4\n"+
		"TYPE F F F I\n"+
		"COUNT 1 1 1 1\n"+
		"WIDTH %d\n"+
		"HEIGHT %d\n"+
		"VIEWPOINT 0 0 0 1 0 0 0\n"+
		"POINTS %d\n"+
		"DATA ascii\n",
		pc.Depth.Width()*pc.Depth.Height(),
		1, //pc.Depth.Height(),
		pc.Depth.Width()*pc.Depth.Height(),
	)

	if err != nil {
		return err
	}

	scale := float32(pc.Depth.max - pc.Depth.min)
	golog.Global.Debugf("min: %d max: %d scale: %f\n", pc.Depth.min, pc.Depth.max, scale)

	for x := 0; x < pc.Depth.Width(); x++ {
		for y := 0; y < pc.Depth.Height(); y++ {
			height := pc.Depth.GetDepth(x, y)
			diff := float32(height - pc.Depth.min)
			scaledHeight := diff / scale

			_, err = fmt.Fprintf(out, "%f %f %f %d\n",
				_scale(x, pc.Depth.Width()),
				_scale(y, pc.Depth.Width()),
				scaledHeight,
				_colorToInt(pc.Color.ColorXY(x, y)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
