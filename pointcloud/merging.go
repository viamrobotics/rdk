package pointcloud

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/lucasb-eyer/go-colorful"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

// CloudAndOffsetFunc is a function that returns a PointCloud with a pose that represents an offset to be applied to every point.
type CloudAndOffsetFunc func(context context.Context) (PointCloud, spatialmath.Pose, error)

// MergePointClouds takes a slice of points clouds with optional offsets and adds all their points to one point cloud.
func MergePointClouds(ctx context.Context, cloudFuncs []CloudAndOffsetFunc, logger golog.Logger) (PointCloud, error) {
	if len(cloudFuncs) == 0 {
		return nil, errors.New("no point clouds to merge")
	}
	finalPoints := make(chan []PointAndData, 50)
	activeReaders := int32(len(cloudFuncs))
	for i, pcSrc := range cloudFuncs {
		iCopy := i
		pcSrcCopy := pcSrc
		utils.PanicCapturingGo(func() {
			_, span := trace.StartSpan(ctx, "pointcloud::MergePointClouds::Cloud"+fmt.Sprint(iCopy))
			defer span.End()

			defer func() {
				atomic.AddInt32(&activeReaders, -1)
			}()
			pc, offset, err := pcSrcCopy(ctx)
			if err != nil {
				panic(err) // TODO(erh) is there something better to do?
			}
			var wg sync.WaitGroup
			const numLoops = 8
			for loop := 0; loop < numLoops; loop++ {
				wg.Add(1)
				f := func(loop int) {
					defer wg.Done()
					const batchSize = 500
					batch := make([]PointAndData, 0, batchSize)
					savedDualQuat := spatialmath.NewZeroPose()
					pc.Iterate(numLoops, loop, func(p r3.Vector, d Data) bool {
						if offset != nil {
							spatialmath.ResetPoseDQTranslation(savedDualQuat, p)
							newPose := spatialmath.Compose(offset, savedDualQuat)
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
	var err error

	firstRead := true     // if all readers finish before loop starts, will prematurely exit, so read at least once.
	dataLastTime := false // there was data in the channel in the previous loop, so continue reading.
	lastRead := false     // read one more time in case all active readers finished while loop was waiting for data.
	for firstRead || lastRead || dataLastTime || atomic.LoadInt32(&activeReaders) > 0 {
		firstRead = false
		logger.Debugf("number of batches in channel: %d\n", len(finalPoints))
		select {
		case ps := <-finalPoints:
			logger.Debugf("number of points in batch: %d\n", len(ps))
			for _, p := range ps {
				if pcTo == nil {
					if p.D == nil {
						pcTo = NewAppendOnlyOnlyPointsPointCloud(len(cloudFuncs) * 640 * 800)
					} else {
						pcTo = NewWithPrealloc(len(cloudFuncs) * 640 * 800)
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
			if lastRead {
				break // this is the 2nd time waiting for data.
			}
			lastRead = true
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
