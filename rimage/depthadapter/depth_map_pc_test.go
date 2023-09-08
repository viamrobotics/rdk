//go:build cgo
package depthadapter_test

import (
	"context"
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

// Intrinsics for the Intel 515 used to capture rimage/board2.dat.gz.
func genIntrinsics() *transform.PinholeCameraIntrinsics {
	return &transform.PinholeCameraIntrinsics{
		Width:  1024,
		Height: 768,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    494.95941428,
		Ppy:    370.70529534,
	}
}

func TestDMPointCloudAdapter(t *testing.T) {
	m, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	adapter := depthadapter.ToPointCloud(m, genIntrinsics())
	test.That(t, adapter, test.ShouldNotBeNil)
	test.That(t, adapter.Size(), test.ShouldEqual, 812049)

	// Test Uncached Iterate
	var xTotalUncached, yTotalUncached, zTotalUncached float64
	adapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		xTotalUncached += p.X
		yTotalUncached += p.Y
		zTotalUncached += p.Z
		return true
	})

	// Test Cached Iterate
	var xTotalCached, yTotalCached, zToatlCached float64
	adapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		xTotalCached += p.X
		yTotalCached += p.Y
		zToatlCached += p.Z
		return true
	})
	test.That(t, xTotalCached, test.ShouldAlmostEqual, xTotalUncached)
	test.That(t, yTotalCached, test.ShouldAlmostEqual, yTotalUncached)
	test.That(t, zToatlCached, test.ShouldAlmostEqual, zTotalUncached)
}

func TestDMPointCloudAdapterRace(t *testing.T) {
	m, err := rimage.NewDepthMapFromFile(context.Background(), artifact.MustPath("rimage/board2_gray.png"))
	test.That(t, err, test.ShouldBeNil)

	baseAdapter := depthadapter.ToPointCloud(m, genIntrinsics())
	raceAdapter := depthadapter.ToPointCloud(m, genIntrinsics())

	var wg sync.WaitGroup
	wg.Add(2)

	utils.PanicCapturingGo(func() {
		meta := raceAdapter.MetaData()
		test.That(t, meta, test.ShouldNotBeNil)
		defer wg.Done()
	})
	utils.PanicCapturingGo(func() {
		defer wg.Done()
		raceAdapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			return true
		})
	})

	wg.Wait()
	test.That(t, raceAdapter.MetaData(), test.ShouldNotBeNil)

	// Make sure that all points in the base adapter are in the race adapter
	baseAdapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		_, isin := raceAdapter.At(p.X, p.Y, p.Z)
		test.That(t, isin, test.ShouldBeTrue)
		return true
	})
	// And Vice Versa
	raceAdapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		_, isin := baseAdapter.At(p.X, p.Y, p.Z)
		test.That(t, isin, test.ShouldBeTrue)
		return true
	})
}
