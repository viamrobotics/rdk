package segmentation

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func init() {
	utils.ParallelFactor = 1
}

type chunkImageDebug struct{}

func (cid *chunkImageDebug) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	imgraw, img2 image.Image,
	logger logging.Logger,
) error {
	t.Helper()
	img := rimage.ConvertImage(imgraw)
	dm, _ := rimage.ConvertImageToDepthMap(context.Background(), img2) // DepthMap is optional, ok if nil.

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
			out, err := ShapeWalkMultiple(img, dm, starts, ShapeWalkOptions{SkipCleaning: true}, logger)
			if err != nil {
				return err
			}
			pCtx.GotDebugImage(out, "shapes-noclean")
		}

		out, err := ShapeWalkMultiple(img, dm, starts, ShapeWalkOptions{}, logger)
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
				_, err := ShapeWalkMultiple(img, dm, []image.Point{s.Start}, ShapeWalkOptions{Debug: true}, logger)
				if err != nil {
					return err
				}
			}
		}
	}

	if true {
		out, err := ShapeWalkEntireDebug(img, dm, ShapeWalkOptions{}, logger)
		if err != nil {
			return err
		}
		pCtx.GotDebugImage(out, "entire")
	}

	if dm != nil {
		x := dm.ToPrettyPicture(0, 0)
		pCtx.GotDebugImage(x, "depth")

		x2 := dm.InterestingPixels(2)
		pCtx.GotDebugImage(x2, "depth-interesting")

		pp := transform.ParallelProjection{}
		pc, err := pp.RGBDToPointCloud(img, dm)
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
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/test1/color", "*.png", "segmentation/test1/depth")
	err := d.Process(t, &chunkImageDebug{})
	test.That(t, err, test.ShouldBeNil)
}
