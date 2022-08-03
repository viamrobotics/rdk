package rimage_test

import (
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

// Intrinsics for the Intel 515 used to capture rimage/board2.dat.gz
func genIntrinsics() *transform.PinholeCameraIntrinsics {
	return &transform.PinholeCameraIntrinsics{
		Width:  1024,
		Height: 768,
		Fx:     821.32642889,
		Fy:     821.68607359,
		Ppx:    494.95941428,
		Ppy:    370.70529534,
		Distortion: transform.DistortionModel{
			RadialK1:     0.11297234,
			RadialK2:     -0.21375332,
			RadialK3:     -0.01584774,
			TangentialP1: -0.00302002,
			TangentialP2: 0.19969297,
		},
	}
}

func TestDMPointCloudAdapter(t *testing.T) {
	m, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	adapter := m.ToPointCloud(genIntrinsics())
	test.That(t, adapter, test.ShouldNotBeNil)
	test.That(t, adapter.Size(), test.ShouldEqual, 812049)

	// Test Uncached Iterate
	var x_total_uncached, y_total_uncached, z_total_uncached float64
	adapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		x_total_uncached += p.X
		y_total_uncached += p.Y
		z_total_uncached += p.Z
		return true
	})

	// Test Cached Iterate
	var x_total_cached, y_total_cached, z_total_cached float64
	adapter.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		x_total_cached += p.X
		y_total_cached += p.Y
		z_total_cached += p.Z
		return true
	})
	test.That(t, x_total_cached, test.ShouldAlmostEqual, x_total_uncached)
	test.That(t, y_total_cached, test.ShouldAlmostEqual, y_total_uncached)
	test.That(t, z_total_cached, test.ShouldAlmostEqual, z_total_uncached)
}

func TestDMPointCloudAdapterRace(t *testing.T) {
	m, err := rimage.ParseDepthMap(artifact.MustPath("rimage/board2.dat.gz"))
	test.That(t, err, test.ShouldBeNil)

	baseAdapter := m.ToPointCloud(genIntrinsics())
	raceAdapter := m.ToPointCloud(genIntrinsics())

	var wg sync.WaitGroup
	wg.Add(2)

	utils.PanicCapturingGo(func() {
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
