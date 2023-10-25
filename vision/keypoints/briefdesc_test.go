package keypoints

import (
	"testing"

	"github.com/fogleman/gg"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestGenerateSamplePairs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	patchSize := 250
	descSize := 128
	offset := (patchSize / 2) - 1
	tempDir := t.TempDir()
	logger.Infof("writing sample points to %s", tempDir)
	// create plotter
	plotTmpImage := func(fileName string, sp *SamplePairs) {
		dc := gg.NewContext(patchSize, patchSize)
		dc.SetRGBA(0, 1, 0, 0.5)
		for i := 0; i < sp.N; i++ {
			dc.SetLineWidth(1.25)
			dc.DrawLine(
				float64(sp.P0[i].X+offset), float64(sp.P0[i].Y+offset),
				float64(sp.P1[i].X+offset), float64(sp.P1[i].Y+offset),
			)
			dc.Stroke()
		}
		dc.SavePNG(tempDir + "/" + fileName)
	}
	// uniform distribution
	uniformDist := GenerateSamplePairs(uniform, descSize, patchSize)
	test.That(t, uniformDist.N, test.ShouldEqual, descSize)
	test.That(t, len(uniformDist.P0), test.ShouldEqual, descSize)
	test.That(t, len(uniformDist.P1), test.ShouldEqual, descSize)
	plotTmpImage("uniform_dist.png", uniformDist)
	// fixed distribution
	fixedDist := GenerateSamplePairs(fixed, descSize, patchSize)
	test.That(t, fixedDist.N, test.ShouldEqual, descSize)
	test.That(t, len(fixedDist.P0), test.ShouldEqual, descSize)
	test.That(t, len(fixedDist.P1), test.ShouldEqual, descSize)
	plotTmpImage("fixed_dist.png", fixedDist)
}
