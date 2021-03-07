package rimage

import (
	"fmt"
	"io"
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
func (pc *ImageWithDepth) ToPCD(out io.Writer) error {
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

	min, max := pc.Depth.MinMax()
	scale := float32(max - min)

	for x := 0; x < pc.Depth.Width(); x++ {
		for y := 0; y < pc.Depth.Height(); y++ {
			height := pc.Depth.GetDepth(x, y)
			diff := float32(height - min)
			scaledHeight := diff / scale

			_, err = fmt.Fprintf(out, "%f %f %f %d\n",
				_scale(x, pc.Depth.Width()),
				_scale(y, pc.Depth.Width()),
				scaledHeight,
				_colorToInt(pc.Color.GetXY(x, y)))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
