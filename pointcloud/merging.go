package pointcloud

import (
	"errors"
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
	"github.com/lucasb-eyer/go-colorful"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

// PointCloudWithOffset has an optional offset that gets applied to every point within the point cloud.
type PointCloudWithOffset struct {
	PointCloud
	Offset spatialmath.Pose
}

// MergePointClouds takes a slice of points clouds with optional offsets and adds all their points to one point cloud.
func MergePointClouds(clouds []PointCloudWithOffset) (PointCloud, error) {
	if len(clouds) == 0 {
		return nil, errors.New("no point clouds to merge")
	}
	if len(clouds) == 1 && clouds[0].Offset == nil {
		return clouds[0], nil
	}
	finalPoints := make(chan []PointAndData, 50)
	activeReaders := int32(len(clouds))
	for i, pc := range clouds {
		iCopy := i
		pcCopy := pc
		utils.PanicCapturingGo(func() {
			ctx, span := trace.StartSpan(ctx, "pointcloud::MergePointClouds::Cloud"+string(i))
			defer span.End()

			defer func() {
				atomic.AddInt32(&activeReaders, -1)
			}()
			var wg sync.WaitGroup
			const numLoops = 8
			for loop := 0; loop < numLoops; loop++ {
				wg.Add(1)
				f := func(loop int) {
					defer wg.Done()
					const batchSize = 500
					batch := make([]PointAndData, 0, batchSize)
					savedDualQuat := spatialmath.NewZeroPose()
					pcSrc.Iterate(numLoops, loop, func(p r3.Vector, d Data) bool {
						if pcCopy.Offset != nil {
							spatialmath.ResetPoseDQTranslation(savedDualQuat, p)
							newPose := spatialmath.Compose(pcCopy.Offset, savedDualQuat)
							p = newPose.Point()
						}
						batch = append(batch, PointAndData{P: p, D: d})
						if len(batch) > batchSize {
							finalPoints <- batch
							batch = make([]PointAndData, 0, batchSize)
						}
						return true
					})
					finalPoints <- batch
				}
				loopCopy := loop
				utils.PanicCapturingGo(func() { f(loopCopy) })
			}
			wg.Wait()
		})
	}
	var pcTo PointCloud

	dataLastTime := false
	for dataLastTime || atomic.LoadInt32(&activeReaders) > 0 {
		select {
		case ps := <-finalPoints:
			for _, p := range ps {
				if pcTo == nil {
					if p.D == nil {
						pcTo = NewAppendOnlyOnlyPointsPointCloud(len(clouds) * 640 * 800)
					} else {
						pcTo = NewWithPrealloc(len(clouds) * 640 * 800)
					}
				}

				myErr := pcTo.Set(p.P, p.D)
				if myErr != nil {
					err = myErr
				}
			}
			dataLastTime = true
		case <-time.After(5 * time.Millisecond):
			dataLastTime = false
			continue
		}
	}

	if err != nil {
		return nil, err
	}

	return pcTo, nil

}

// MergePointCloudsWithColor creates a union of point clouds from the slice of point clouds, giving
// each element of the slice a unique color.
func MergePointCloudsWithColor(clusters []PointCloud) (PointCloud, error) {
	var err error
	palette := colorful.FastWarmPalette(len(clusters))
	colorSegmentation := New()
	for i, cluster := range clusters {
		col, ok := color.NRGBAModel.Convert(palette[i]).(color.NRGBA)
		if !ok {
			panic("impossible")
		}
		cluster.Iterate(0, 0, func(v r3.Vector, d Data) bool {
			err = colorSegmentation.Set(v, NewColoredData(col))
			return err == nil
		})
		if err != nil {
			return nil, err
		}
	}
	return colorSegmentation, nil
}
