package segmentation

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

const debugChunks = "VIAM_DEBUG"

func init() {
	utils.ParallelFactor = 1
}

type chunkImageDebug struct{}

func (cid *chunkImageDebug) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	imgraw image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	iwd := rimage.ConvertToImageWithDepth(imgraw)
	img := iwd.Color

	type AShape struct {
		Start      image.Point
		PixelRange []int
		BadPoints  []image.Point
	}

	type imgConfig struct {
		Shapes []AShape
	}

	cfg := imgConfig{}
	err := pCtx.CurrentImgConfig(&cfg)
	if err != nil {
		return err
	}

	if true {
		out := img.InterestingPixels(.2)
		pCtx.GotDebugImage(out, "t")
	}

	if true {
		starts := []image.Point{}

		for _, s := range cfg.Shapes {
			starts = append(starts, s.Start)
		}

		if true {
			// this shows things with the cleaning, is it useful, not sure
			out, err := ShapeWalkMultiple(iwd, starts, ShapeWalkOptions{SkipCleaning: true}, logger)
			if err != nil {
				return err
			}
			pCtx.GotDebugImage(out, "shapes-noclean")
		}

		out, err := ShapeWalkMultiple(iwd, starts, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}

		pCtx.GotDebugImage(out, "shapes")

		for idx, s := range cfg.Shapes {
			numPixels := out.PixelsInSegmemnt(idx + 1)

			reRun := false

			if numPixels < s.PixelRange[0] || numPixels > s.PixelRange[1] {
				reRun = true
				t.Errorf("out of pixel range %s %v %d", fn, s, numPixels)
			}

			for _, badPoint := range s.BadPoints {
				if out.GetSegment(badPoint) == idx+1 {
					reRun = true
					t.Errorf("point %v was in cluster %v but should not have been", badPoint, idx+1)
				}
			}

			if reRun {
				// run again with debugging on
				_, err := ShapeWalkMultiple(iwd, []image.Point{s.Start}, ShapeWalkOptions{Debug: true}, logger)
				if err != nil {
					return err
				}
			}
		}
	}

	if true {
		out, err := ShapeWalkEntireDebug(iwd, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}
		pCtx.GotDebugImage(out, "entire")
	}

	if iwd.Depth != nil {
		x := iwd.Depth.ToPrettyPicture(0, 0)
		pCtx.GotDebugImage(x, "depth")

		x2 := iwd.Depth.InterestingPixels(2)
		pCtx.GotDebugImage(x2, "depth-interesting")

		pp := rimage.ParallelProjection{}
		pc, err := pp.ImageWithDepthToPointCloud(iwd)
		if err != nil {
			t.Fatal(err)
		}

		plane, removed, err := SegmentPlane(context.Background(), pc, 3000, 5)
		if err != nil {
			t.Fatal(err)
		}

		planePc, err := plane.PointCloud()
		if err != nil {
			t.Fatal(err)
		}
		pCtx.GotDebugPointCloud(planePc, "only-plane")
		pCtx.GotDebugPointCloud(removed, "plane-removed")
	}

	return nil
}

func TestChunk1(t *testing.T) {
	chunkTest := os.Getenv(debugChunks)
	if chunkTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugChunks)
	}
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/test1", "*", true)
	err := d.Process(t, &chunkImageDebug{})
	test.That(t, err, test.ShouldBeNil)
}
