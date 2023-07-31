// Package obstacledepth uses an underlying depth camera to fulfill GetObjectPointClouds,
// using the method outlined in ()
package obstacledepth

import (
	"context"
	"image"
	"math"
	"runtime"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	svision "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
	vision "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/segmentation"
)

var model = resource.DefaultModelFamily.WithModel("obstacle_depth")

// ObstaclesDepthConfig specifies the parameters for the camera to be used
// for the obstacle distance detection service.
type ObstaclesDepthConfig struct {
	K          int     `json:"k"`
	Hmin       float64 `json:"hmin"`
	Hmax       float64 `json:"hmax"`
	ThetaMax   float64 `json:"theta_max"`
	ReturnPCDs bool    `json:"return_pcds"`
	intrinsics *transform.PinholeCameraIntrinsics
}

type obsPoint struct {
	point      image.Point
	isObstacle bool
	depth      rimage.Depth
}

type obsDepth struct {
	dm         *rimage.DepthMap
	ptChunks   [][]obsPoint
	ptList     []obsPoint
	hMin       float64
	hMax       float64
	sinTheta   float64
	intrinsics *transform.PinholeCameraIntrinsics
	returnPCDs bool
	k          int
}

const (
	// params from paper (def need to link paper somewhere in here shoutout them).
	defaultHmin     = 0.0
	defaultHmax     = 150.0
	defaultThetamax = math.Pi / 4
	chunkSize       = 200 // we send chunkSize points in each goroutine to speed things up
	sampleN         = 4   // we sample 1 in every sampleN depth points to speed things up (lol)
	maxThreads      = 300 // we will run at mos maxThreads goroutines.. to speed things up
)

func init() {
	resource.RegisterService(svision.API, model, resource.Registration[svision.Service, *ObstaclesDepthConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (svision.Service, error) {
			attrs, err := resource.NativeConfig[*ObstaclesDepthConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerObstacleDepth(ctx, c.ResourceName(), attrs, actualR)
		},
	})
}

// Validate ensures all parts of the config are valid.
func (config *ObstaclesDepthConfig) Validate(path string) ([]string, error) {
	deps := []string{}
	if config.K < 1 || config.K > 50 {
		return nil, errors.New("invalid K, pick an integer between 1 and 50 (10 recommended)")
	}
	if (config.Hmin >= config.Hmax) && config.Hmax != 0 {
		return nil, errors.New("Hmin should be less than Hmax")
	}
	return deps, nil
}

func registerObstacleDepth(
	ctx context.Context,
	name resource.Name,
	conf *ObstaclesDepthConfig,
	r robot.Robot,
) (svision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDistanceDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacle_depth cannot be nil")
	}

	// If you have no intrinsics, you get the dumb, quick version of obstacles_depth
	if conf.intrinsics == nil {
		r.Logger().Warn("you're doing it the dumb way without intrinsics but okaaaay")
		segmenter := func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
			// Return the shortest depth in the depth map as a Geometry point
			depthStream, err := src.Stream(ctx)
			if err != nil {
				return nil, errors.Errorf("could not get stream from %s", src)
			}
			pic, release, err := depthStream.Next(ctx)
			if err != nil {
				return nil, errors.Errorf("could not get image from stream %s", depthStream)
			} // maybe try again real quick somehow
			defer release()
			// Get the data from the depth map
			dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
			if err != nil {
				return nil, errors.New("could not convert image to depth map")
			}
			min, _ := dm.MinMax()

			pt := spatialmath.NewPoint(r3.Vector{X: 0, Y: 0, Z: float64(min)}, "obstacle")
			toReturn := make([]*vision.Object, 1)
			toReturn[0] = &vision.Object{Geometry: pt}

			return toReturn, nil
		}
		return svision.NewService(name, r, nil, nil, nil, segmenter)
	}

	// Use defaults if needed
	if conf.Hmax == 0 {
		conf.Hmax = defaultHmax
	}
	if conf.ThetaMax == 0 {
		conf.ThetaMax = defaultThetamax
	}

	myObsDep := obsDepth{
		hMin: conf.Hmin, hMax: conf.Hmax, sinTheta: math.Sin(conf.ThetaMax),
		intrinsics: conf.intrinsics, returnPCDs: conf.ReturnPCDs,
	}
	segmenter := myObsDep.buildObsDepthWithIntrinsics() // does the thing
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

func (o *obsDepth) buildObsDepthWithIntrinsics() segmentation.Segmenter {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		depthStream, err := src.Stream(ctx)
		if err != nil {
			return nil, errors.Errorf("could not get stream from %s", src)
		}
		pic, release, err := depthStream.Next(ctx)
		if err != nil {
			return nil, errors.Errorf("could not get image from stream %s", depthStream)
		} // maybe try again real quick somehow
		defer release()
		// Get the data from the depth map
		dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
		if err != nil {
			return nil, errors.New("could not convert image to depth map")
		}
		o.dm = dm
		o.makePointList(sampleN)
		runtime.GOMAXPROCS(maxThreads)

		// Use some goroutines to determine if each depth pixel is an obstacle
		doneCh := make(chan bool, len(o.ptChunks))
		var wg sync.WaitGroup
		for i, chunk := range o.ptChunks {
			wg.Add(1)
			go func(i int, chunk []obsPoint) {
				defer wg.Done()
				for j, p := range chunk {
					o.ptChunks[i][j].isObstacle = o.isObstaclePoint(p.point)
				}
				doneCh <- true
			}(i, chunk)
		}
		go func() {
			wg.Wait()
			close(doneCh)
		}()
		count := 0
		for c := range doneCh {
			if c {
				count++
			}
		}

		// Cluster on the 2D depth points and then project the 2D clusters into 3D boxes
		outClusters, err := o.performKMeans(o.k)
		if err != nil {
			return nil, err
		}
		boxes, err := o.clustersToBoxes(outClusters)
		if err != nil {
			return nil, err
		}

		// Packaging the return depending on if they want PCDs
		n := int(math.Min(float64(len(outClusters)), float64(len(boxes)))) // should be same len but for safety
		toReturn := make([]*vision.Object, n)
		for i := 0; i < n; i++ { // for each cluster/box make an object
			if o.returnPCDs {
				pcdToReturn := pointcloud.New()
				basicData := pointcloud.NewBasicData()
				for _, pt := range outClusters[i].Observations {
					if len(pt.Coordinates()) >= 3 {
						vec := r3.Vector{X: pt.Coordinates()[0], Y: pt.Coordinates()[1], Z: pt.Coordinates()[2]}
						err = pcdToReturn.Set(vec, basicData)
						if err != nil {
							return nil, err
						}
					}
				}
				toReturn[i] = &vision.Object{PointCloud: pcdToReturn, Geometry: boxes[i]}
			} else {
				toReturn[i] = &vision.Object{Geometry: boxes[i]}
			}
		}
		return toReturn, nil
	}
}

// Grab points from the depthmap. Sample every n.
func (o *obsDepth) makePointList(n int) {
	w, h := o.dm.Width(), o.dm.Height()
	out := make([]obsPoint, 0, w*h)
	for i := 0; i < w; i += n {
		for j := 0; j < h; j++ {
			out = append(out, obsPoint{point: image.Pt(i, j), depth: o.dm.GetDepth(i, j)})
		}
	}
	o.ptList = out
	o.ptChunks = splitPtList(out, chunkSize)
}

// Split the pointlist into a list of lists, len(chunk) = chunkSize (not the last one).
func splitPtList(slice []obsPoint, chunkSize int) [][]obsPoint {
	var chunks [][]obsPoint
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize

		// necessary check to avoid slicing beyond
		// slice capacity
		if end > len(slice) {
			end = len(slice)
		}

		chunks = append(chunks, slice[i:end])
	}

	return chunks
}

// Returns true/false if compatible with another point in the depthmap.
func (o *obsDepth) isObstaclePoint(point image.Point) bool {
	for _, p := range o.ptList {
		if point == p.point {
			continue
		}
		if o.isCompatible(point, p.point) {
			return true
		}
	}
	return false
}

// Check compatibility between 2 points.
func (o *obsDepth) isCompatible(p1, p2 image.Point) bool {
	// thetaMax in radians
	xdist, ydist := math.Abs(float64(p1.X-p2.X)), math.Abs(float64(p1.Y-p2.Y))
	zdist := math.Abs(float64(o.dm.Get(p1)) - float64(o.dm.Get(p2)))
	dist := math.Sqrt((xdist * xdist) + (ydist * ydist) + (zdist * zdist))

	if ydist < o.hMin || ydist > o.hMax {
		return false
	}
	if ydist/dist < o.sinTheta {
		return false
	}
	return true
}

// Turn the clusters we get from kmeans into boxes..
func (o *obsDepth) clustersToBoxes(clusters clusters.Clusters) ([]spatialmath.Geometry, error) {
	boxes := make([]spatialmath.Geometry, 0, len(clusters))

	for i, c := range clusters {
		var xmax, ymax, zmax float64
		xmin, ymin, zmin := math.Inf(1), math.Inf(1), math.Inf(1)

		for _, pt := range c.Observations {
			u, v := pt.Coordinates().Coordinates()[0], pt.Coordinates().Coordinates()[1]
			x, y, z := o.intrinsics.PixelToPoint(u, v, float64(o.dm.GetDepth(int(u), int(v))))

			// Lol is this the best I can do?
			if x < xmin {
				xmin = x
			}
			if x > xmax {
				xmax = x
			}
			if y < ymin {
				ymin = y
			}
			if y > ymax {
				ymax = y
			}
			if z < zmin {
				zmin = z
			}
			if z > zmax {
				zmax = z
			}
		}
		// Make a box from those bounds no matter what they are and add it in
		xdiff, ydiff, zdiff := xmax-xmin, ymax-ymin, zmax-zmin
		xc, yc, zc := (xmin+xmax)/2, (ymin+ymax)/2, (zmin+zmax)/2
		pose := spatialmath.NewPose(r3.Vector{xc, yc, zc}, spatialmath.NewZeroOrientation())

		box, err := spatialmath.NewBox(pose, r3.Vector{xdiff, ydiff, zdiff}, strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		boxes = append(boxes, box)
	}
	return boxes, nil
}

// Do Kmeans clustering on all the 2D obstacle points.
func (o *obsDepth) performKMeans(k int) (clusters.Clusters, error) {
	var d clusters.Observations
	for _, pt := range o.ptList {
		if pt.isObstacle {
			d = append(d, clusters.Coordinates{float64(pt.point.X), float64(pt.point.Y)})
		}
	}
	km := kmeans.New()
	return km.Partition(d, k)
}
