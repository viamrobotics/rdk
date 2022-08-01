package pointcloud

import (
	"image/color"
	"math/rand"
	"sync"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"
)

func testPointCloudStorage(t *testing.T, ms storage) {
	t.Helper()

	var point r3.Vector
	var data, gotData Data
	var found bool
	// Empty
	test.That(t, ms.Size(), test.ShouldEqual, 0)
	// Iterate on Empty
	testPointCloudIterate(t, ms, 0, r3.Vector{})
	testPointCloudIterate(t, ms, 4, r3.Vector{})

	// Insertion
	point = r3.Vector{1, 2, 3}
	data = NewColoredData(color.NRGBA{255, 124, 43, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 1)
	gotData, found = ms.At(1, 2, 3)
	test.That(t, found, test.ShouldEqual, true)
	test.That(t, gotData, test.ShouldEqual, data)

	// Second Insertion
	point = r3.Vector{4, 2, 3}
	data = NewColoredData(color.NRGBA{232, 111, 75, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 2)

	// Insertion of duplicate point
	data = NewColoredData(color.NRGBA{22, 1, 78, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 2)
	gotData, found = ms.At(4, 2, 3)
	test.That(t, found, test.ShouldEqual, true)
	test.That(t, gotData, test.ShouldEqual, data)

	// Retrieval of non-existent point
	gotData, found = ms.At(3, 1, 7)
	test.That(t, found, test.ShouldEqual, false)
	test.That(t, gotData, test.ShouldBeNil)

	// Iteration
	ms.Set(r3.Vector{3, 1, 7}, NewColoredData(color.NRGBA{22, 1, 78, 255}))
	expectedCentroid := r3.Vector{8 / 3.0, 5 / 3.0, 13 / 3.0}

	// Zero batches
	testPointCloudIterate(t, ms, 0, expectedCentroid)

	// One batch
	testPointCloudIterate(t, ms, 1, expectedCentroid)

	// Batches equal to the number of points
	testPointCloudIterate(t, ms, ms.Size(), expectedCentroid)

	// Batches greater than the number of points
	testPointCloudIterate(t, ms, ms.Size()*2, expectedCentroid)
}

func testPointCloudIterate(t *testing.T, ms storage, numBatches int, expectedCentroid r3.Vector) {
	t.Helper()

	if numBatches == 0 {
		var totalX, totalY, totalZ float64
		count := 0
		ms.Iterate(0, 0, func(p r3.Vector, d Data) bool {
			totalX += p.X
			totalY += p.Y
			totalZ += p.Z
			count++
			return true
		})
		test.That(t, count, test.ShouldEqual, ms.Size())
		if count == 0 {
			test.That(t, totalX, test.ShouldEqual, 0)
			test.That(t, totalY, test.ShouldEqual, 0)
			test.That(t, totalZ, test.ShouldEqual, 0)
		} else {
			test.That(t, totalX/float64(count), test.ShouldAlmostEqual, expectedCentroid.X)
			test.That(t, totalY/float64(count), test.ShouldAlmostEqual, expectedCentroid.Y)
			test.That(t, totalZ/float64(count), test.ShouldAlmostEqual, expectedCentroid.Z)
		}
	} else {
		var totalX, totalY, totalZ float64
		var count int
		var wg sync.WaitGroup
		wg.Add(numBatches)
		totalXChan := make(chan float64, numBatches)
		totalYChan := make(chan float64, numBatches)
		totalZChan := make(chan float64, numBatches)
		countChan := make(chan int, numBatches)
		for loop := 0; loop < numBatches; loop++ {
			f := func(myBatch int) {
				defer wg.Done()
				var totalXBuf, totalYBuf, totalZBuf float64
				var countBuf int
				ms.Iterate(numBatches, myBatch, func(p r3.Vector, d Data) bool {
					totalXBuf += p.X
					totalYBuf += p.Y
					totalZBuf += p.Z
					countBuf++
					return true
				})
				totalXChan <- totalXBuf
				totalYChan <- totalYBuf
				totalZChan <- totalZBuf
				countChan <- countBuf
			}
			loopCopy := loop
			utils.PanicCapturingGo(func() { f(loopCopy) })
		}
		wg.Wait()
		for loop := 0; loop < numBatches; loop++ {
			totalX += <-totalXChan
			totalY += <-totalYChan
			totalZ += <-totalZChan
			count += <-countChan
		}
		test.That(t, count, test.ShouldEqual, ms.Size())
		if count == 0 {
			test.That(t, totalX, test.ShouldEqual, 0)
			test.That(t, totalY, test.ShouldEqual, 0)
			test.That(t, totalZ, test.ShouldEqual, 0)
		} else {
			test.That(t, totalX/float64(count), test.ShouldAlmostEqual, expectedCentroid.X)
			test.That(t, totalY/float64(count), test.ShouldAlmostEqual, expectedCentroid.Y)
			test.That(t, totalZ/float64(count), test.ShouldAlmostEqual, expectedCentroid.Z)
		}
	}
}

func benchPointCloudStorage(b *testing.B, ms storage) {
	b.Helper()

	pc_max := 10_000.
	for i := 0; i < b.N; i++ {
		rand.Seed(0)
		pointList := make([]PointAndData, 0, 10_000)
		for j := 0; j < cap(pointList); j++ {
			pointList = append(pointList, PointAndData{r3.Vector{rand.Float64() * pc_max, rand.Float64() * pc_max, rand.Float64() * pc_max},
				NewColoredData(color.NRGBA{uint8(rand.Intn(256)), uint8(rand.Intn(256)), uint8(rand.Intn(256)), 255})})
		}
		// Set all points
		for _, p := range pointList {
			ms.Set(p.P, p.D)
		}
		// Retrieve all points
		for _, p := range pointList {
			_, found := ms.At(p.P.X, p.P.Y, p.P.Z)
			if !found {
				b.Errorf("Point %v not found", p.P)
			}
		}
		// Overwrite all points
		for _, p := range pointList {
			ms.Set(p.P, p.D)
		}
	}
}
