package rimage

import (
	"fmt"
	"io"

	"go.viam.com/robotcore/pointcloud"
)

func _colorToInt(c Color) int {
	r, g, b := c.RGB255()
	x := 0

	x = x | (int(r) << 16)
	x = x | (int(g) << 8)
	x = x | (int(b) << 0)
	//x = x | ( int(c.A) << 0 )

	return x
}

func _scale(x, max int) float32 {
	res := float32(x) / float32(max)
	return res - .5
}
func (iwd *ImageWithDepth) ToPCD(out io.Writer) error {
	if iwd.Depth == nil {
		return fmt.Errorf("no depth data")
	}

	if iwd.Depth.Width() != iwd.Color.Width() ||
		iwd.Depth.Height() != iwd.Color.Height() {
		return fmt.Errorf("depth map and color dimensions don't match %d,%d -> %d,%d",
			iwd.Depth.Width(), iwd.Depth.Height(), iwd.Color.Width(), iwd.Color.Height())
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
		iwd.Depth.Width()*iwd.Depth.Height(),
		1, //iwd.Depth.Height(),
		iwd.Depth.Width()*iwd.Depth.Height(),
	)

	if err != nil {
		return err
	}

	min, max := iwd.Depth.MinMax()
	scale := float32(max - min)

	for x := 0; x < iwd.Depth.Width(); x++ {
		for y := 0; y < iwd.Depth.Height(); y++ {
			height := iwd.Depth.GetDepth(x, y)
			diff := float32(height - min)
			scaledHeight := diff / scale

			_, err = fmt.Fprintf(out, "%f %f %f %d\n",
				_scale(x, iwd.Depth.Width()),
				_scale(y, iwd.Depth.Width()),
				scaledHeight,
				_colorToInt(iwd.Color.GetXY(x, y)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Naive implementation that does not take into account camera parameters
// If you have the Camera matrices, use calib.DepthMapToPointCloud
func (dm *DepthMap) ToPointCloud() (*pointcloud.PointCloud, error) {
	pc := pointcloud.New()

	height := dm.Height()
	width := dm.Width()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			err := pc.Set(pointcloud.NewBasicPointInt(x, y, int(z)))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}
