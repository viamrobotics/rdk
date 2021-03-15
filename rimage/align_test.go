package rimage

import (
	"context"
	"fmt"
	"image"
	"math"
	"testing"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/testutils"
)

type alignTestHelper struct {
	attrs api.AttributeMap
}

func (h *alignTestHelper) Process(d *MultipleImageTestDebugger, fn string, img image.Image) error {
	ii := ConvertToImageWithDepth(img)

	d.GotDebugImage(ii.Depth.ToPrettyPicture(0, MaxDepth), "depth")

	dc, err := NewDepthComposed(nil, nil, h.attrs)
	if err != nil {
		d.T.Fatal(err)
	}

	fixed, err := dc.alignColorAndDepth(context.TODO(), ii)
	if err != nil {
		d.T.Fatal(err)
	}

	d.GotDebugImage(fixed.Color, "color-fixed")
	d.GotDebugImage(fixed.Depth.ToPrettyPicture(0, MaxDepth), "depth-fixed")

	d.GotDebugImage(fixed.Overlay(), "overlay")
	return nil
}

func TestAlignIntel(t *testing.T) {
	d := NewMultipleImageTestDebugger(t, "align/intel515", "*.both.gz")
	err := d.Process(&alignTestHelper{api.AttributeMap{"config": &intelConfig}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAlignGripper(t *testing.T) {
	config, err := api.ReadConfig(testutils.ResolveFile("robots/configs/gripper-cam.json"))
	if err != nil {
		t.Fatal(err)
	}

	c := config.FindComponent("combined")
	if c == nil {
		t.Fatal("no combined")
	}

	d := NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz")
	err = d.Process(&alignTestHelper{c.Attributes})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAlignDetermine(t *testing.T) {
	inBounds := func(size image.Point, pts []image.Point) bool {
		for _, p := range pts {
			if p.X < 0 || p.Y < 0 {
				return false
			}
			if p.X >= size.X {
				return false
			}
			if p.Y >= size.Y {
				return false
			}
		}

		return true
	}

	expand := func(pts []image.Point, x bool) []image.Point {
		//fmt.Printf("\t 1 : %v %v\n", pts, x)
		center := Center(pts, 100000)
		//fmt.Printf("\t\t %v\n", center)
		n := []image.Point{}
		for _, p := range pts {
			if x {
				dis := center.X - p.X
				newDis := int(float64(dis) * 1.1)
				if dis == newDis {
					newDis = dis * 2
				}
				//fmt.Printf("\t\t\t %v -> %v\n", dis, newDis)
				n = append(n, image.Point{center.X - newDis, p.Y})
			} else {
				dis := center.Y - p.Y
				newDis := int(float64(dis) * 1.1)
				if dis == newDis {
					newDis = dis * 2
				}
				n = append(n, image.Point{p.X, center.Y - newDis})
			}

		}
		return n
	}

	fixPoints := func(pts []image.Point) []image.Point {
		r := BoundingBox(pts)
		return arrayToPoints([]image.Point{r.Min, r.Max})
	}

	foo := func(
		color image.Point, colorPoints []image.Point,
		depth image.Point, depthPoints []image.Point,
	) *alignConfig {
		// this only works for things on a multiple of 90 degrees apart, not arbitrary

		// firse we figure out if we are rotated 90 degrees or not to know which direction to expand
		colorAngle := PointAngle(colorPoints[0], colorPoints[1])
		depthAngle := PointAngle(depthPoints[0], depthPoints[1])

		if colorAngle < 0 {
			colorAngle += math.Pi
		}
		if depthAngle < 0 {
			depthAngle += math.Pi
		}

		colorAngle /= (math.Pi / 2)
		depthAngle /= (math.Pi / 2)

		rotated := false
		if colorAngle < 1 && depthAngle > 1 || colorAngle > 1 && depthAngle < 1 {
			rotated = true
		}

		fmt.Printf("colorAngle: %v depthAngle: %v rotated: %v\n", colorAngle, depthAngle, rotated)

		// now we expand in one direction
		for {
			c2 := expand(colorPoints, true)
			d2 := expand(depthPoints, !rotated)

			if !inBounds(color, c2) || !inBounds(depth, d2) {
				break
			}
			colorPoints = c2
			depthPoints = d2
			fmt.Printf("A: %v %v\n", colorPoints, depthPoints)
		}

		// now we expand in the other direction
		for {
			c2 := expand(colorPoints, false)
			d2 := expand(depthPoints, rotated)

			if !inBounds(color, c2) || !inBounds(depth, d2) {
				break
			}
			colorPoints = c2
			depthPoints = d2
			fmt.Printf("B: %v %v\n", colorPoints, depthPoints)
		}

		colorPoints = fixPoints(colorPoints)
		depthPoints = fixPoints(depthPoints)

		if rotated {
			// TODO(erh): handle flipped
			depthPoints = append(depthPoints[1:], depthPoints[0])
		}

		config := &alignConfig{
			ColorInputSize:  color,
			ColorWarpPoints: arrayToPoints(colorPoints),
			DepthInputSize:  depth,
			DepthWarpPoints: arrayToPoints(depthPoints),
			OutputSize:      image.Point{100, 100},
		}

		fmt.Printf("ColorWarpPoints %v\n", config.ColorWarpPoints)

		return config
	}

	config := foo(
		image.Point{1024, 768},
		[]image.Point{{645, 587}, {713, 111}},
		image.Point{224, 171},
		[]image.Point{{86, 120}, {204, 135}},
	)

	d := NewMultipleImageTestDebugger(t, "align/gripper1", "*.both.gz")
	err := d.Process(&alignTestHelper{api.AttributeMap{"config": config}})
	if err != nil {
		t.Fatal(err)
	}

}
