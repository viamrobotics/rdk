package keypoints

import (
	"go.viam.com/test"
	"testing"
)

func TestLoadORBConfiguration(t *testing.T) {
	cfg := LoadORBConfiguration("orbconfig.json")
	test.That(t, cfg, test.ShouldNotBeNil)
	test.That(t, cfg.Layers, test.ShouldEqual, 4)
	test.That(t, cfg.DownscaleFactor, test.ShouldEqual, 2)
	test.That(t, cfg.FastConf.Threshold, test.ShouldEqual, 0.15)
	test.That(t, cfg.FastConf.NMatchesCircle, test.ShouldEqual, 9)
	test.That(t, cfg.FastConf.NMSWinSize, test.ShouldEqual, 7)
}
