package vision

import (
	"fmt"
	"image/color"
	"io"
)

type PointCloud struct {
	Depth DepthMap
	Color Image
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
	res = res - .5
	return res / 5
}
func (pc *PointCloud) ToPCD(out io.Writer) error {
	if pc.Depth.Width() != pc.Color.Width() ||
		pc.Depth.Height() != pc.Color.Height() {
		return fmt.Errorf("DepthMap and color dimensions don't match %d,%d -> %d,%d",
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
		pc.Depth.Width()*pc.Depth.Height())

	if err != nil {
		return err
	}

	scale := float32(pc.Depth.max - pc.Depth.min)
	fmt.Printf("min: %d max: %d scale: %f\n", pc.Depth.min, pc.Depth.max, scale)

	for x := 0; x < pc.Depth.Width(); x++ {
		for y := 0; y < pc.Depth.Height(); y++ {
			height := pc.Depth.GetDepth(x, y)
			if height > pc.Depth.max {
				fmt.Printf("Wtf: %d\n", height)
			}
			if height < pc.Depth.min {
				fmt.Printf("wtf: %d\n", height)
			}

			diff := float32(height - pc.Depth.min)
			if diff < 0 {
				fmt.Printf("wtf: %d %f\n", height, diff)
			}
			if diff > scale {
				fmt.Printf("wtf: %f\n", diff)
			}

			scaledHeight := diff / scale

			if scaledHeight < 0 || scaledHeight > 1 {
				fmt.Printf("Wtf: %d -> %f\n", height, scaledHeight)
			}

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
