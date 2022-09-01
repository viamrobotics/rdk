package keypoints

import (
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/fogleman/gg"
	"go.viam.com/test"
)

func TestGenerateSamplePairs(t *testing.T) {
	logger := golog.NewTestLogger(t)
	patchSize := 250
	descSize := 128
	sp := GenerateSamplePairs(uniform, descSize, patchSize)
	test.That(t, sp.N, test.ShouldEqual, descSize)
	test.That(t, len(sp.P0), test.ShouldEqual, descSize)
	test.That(t, len(sp.P1), test.ShouldEqual, descSize)
	tempDir, err := os.MkdirTemp("", "brief_uniform_sampling")
	test.That(t, err, test.ShouldBeNil)
	//defer os.RemoveAll(tempDir)
	logger.Infof("writing sample points to %s", tempDir)
	dc := gg.NewContext(patchSize, patchSize)
	dc.SetRGBA(0, 1, 0, 0.5)
	offset := (patchSize / 2) - 1
	for i := 0; i < sp.N; i++ {
		if i%2 == 0 {
			continue
		}
		dc.SetLineWidth(1.25)
		dc.DrawLine(
			float64(sp.P0[i].X+offset), float64(sp.P0[i].Y+offset),
			float64(sp.P1[i].X+offset), float64(sp.P1[i].Y+offset),
		)
		dc.Stroke()
	}
	dc.SavePNG(tempDir + "/sample_pairs.png")
}
