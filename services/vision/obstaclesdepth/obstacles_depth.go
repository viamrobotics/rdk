// Package obstaclesdepth uses an underlying depth camera to fulfill GetObjectPointClouds,
// using the method outlined in (Manduchi, Roberto, et al. "Obstacle detection and terrain classification
// for autonomous off-road navigation." Autonomous robots 18 (2005): 81-102.)
package obstaclesdepth

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	"image"
	"math"
	"strconv"

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

var model = resource.DefaultModelFamily.WithModel("obstacles_depth")

// ObsDepthConfig specifies the parameters to be used for the obstacle depth service.
type ObsDepthConfig struct {
	K          int                                `json:"k"`
	Hmin       float64                            `json:"hmin"`
	Hmax       float64                            `json:"hmax"`
	ThetaMax   float64                            `json:"theta_max"`
	ReturnPCDs bool                               `json:"return_pcds"`
	Intrinsics *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
}

// obsDepth is the underlying struct actually used by the service.
type obsDepth struct {
	dm          *rimage.DepthMap
	obstaclePts []image.Point
	hMin        float64
	hMax        float64
	sinTheta    float64
	intrinsics  *transform.PinholeCameraIntrinsics
	returnPCDs  bool
	k           int
	depthStream gostream.VideoStream
}

const (
	// the first 3 consts are parameters from Manduchi et al.
	defaultHmin     = 0.0
	defaultHmax     = 150.0
	defaultThetamax = math.Pi / 4

	defaultK = 10 // default number of obstacle segments to create
	sampleN  = 8  // we sample 1 in every sampleN depth points
)

func init() {
	resource.RegisterService(svision.API, model, resource.Registration[svision.Service, *ObsDepthConfig]{
		DeprecatedRobotConstructor: func(ctx context.Context, r any, c resource.Config, logger golog.Logger) (svision.Service, error) {
			attrs, err := resource.NativeConfig[*ObsDepthConfig](c)
			if err != nil {
				return nil, err
			}
			actualR, err := utils.AssertType[robot.Robot](r)
			if err != nil {
				return nil, err
			}
			return registerObstaclesDepth(ctx, c.ResourceName(), attrs, actualR, logger)
		},
	})
}

// Validate ensures all parts of the config are valid.
func (config *ObsDepthConfig) Validate(path string) ([]string, error) {
	deps := []string{}
	if config.K < 1 || config.K > 50 {
		return nil, errors.New("invalid K, pick an integer between 1 and 50 (10 recommended)")
	}
	if config.Hmin >= config.Hmax {
		return nil, errors.New("Hmin should be less than Hmax")
	}
	if config.Hmin < 0 {
		return nil, errors.New("Hmin should be greater than or equal to 0")
	}
	if config.Hmax < 0 {
		return nil, errors.New("Hmax should be greater than or equal to 0")
	}
	return deps, nil
}

func registerObstaclesDepth(
	ctx context.Context,
	name resource.Name,
	conf *ObsDepthConfig,
	r robot.Robot,
	logger golog.Logger,
) (svision.Service, error) {
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDistanceDetector")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacles_depth cannot be nil")
	}

	// If you have no intrinsics
	if conf.Intrinsics == nil {
		logger.Warn("obstacles depth started without camera's intrinsic parameters")
		segmenter := buildObsDepthNoIntrinsics()
		return svision.NewService(name, r, nil, nil, nil, segmenter)
	}

	// Use defaults if needed
	if conf.Hmax == 0 {
		conf.Hmax = defaultHmax
	}
	if conf.ThetaMax == 0 {
		conf.ThetaMax = defaultThetamax
	}
	if conf.K == 0 {
		conf.K = defaultK
	}

	myObsDep := obsDepth{
		hMin: conf.Hmin, hMax: conf.Hmax, sinTheta: math.Sin(conf.ThetaMax),
		intrinsics: conf.Intrinsics, returnPCDs: conf.ReturnPCDs, k: conf.K,
	}
	segmenter := myObsDep.buildObsDepthWithIntrinsics() // does the thing
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

// buildObsDepthNoIntrinsics will return the shortest depth in the depth map as a Geometry point.
func buildObsDepthNoIntrinsics() segmentation.Segmenter {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		pic, release, err := camera.ReadImage(ctx, src)
		if err != nil {
			return nil, errors.Errorf("could not get image from %s", src)
		}
		defer release()

		dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
		if err != nil {
			return nil, errors.New("could not convert image to depth map")
		}
		min, _ := dm.MinMax()

		pt := spatialmath.NewPoint(r3.Vector{X: 0, Y: 0, Z: float64(min)}, "")
		toReturn := make([]*vision.Object, 1)
		toReturn[0] = &vision.Object{Geometry: pt}

		return toReturn, nil
	}
}

// buildObsDepthWithIntrinsics will use the methodology in Manduchi et al. to find obstacle points
// before clustering and projecting those points into 3D obstacles.
func (o *obsDepth) buildObsDepthWithIntrinsics() segmentation.Segmenter {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		// Check if we have intrinsics here or in the camera properties. If not, don't even try
		props, err := src.Properties(ctx)
		if err != nil {
			return nil, err
		}
		if props.IntrinsicParams != nil {
			o.intrinsics = props.IntrinsicParams
		}
		if o.intrinsics == nil {
			return nil, errors.New("tried to build obstacles depth with intrinsics but no instrinsics found")
		}
		if o.depthStream == nil {
			depthStream, err := src.Stream(ctx)
			if err != nil {
				return nil, errors.Errorf("could not get stream from %s", src)
			}
			o.depthStream = depthStream
		}

		pic, release, err := o.depthStream.Next(ctx)
		if err != nil {
			return nil, errors.Errorf("could not get image from stream %s", o.depthStream)
		}
		defer release()
		dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
		if err != nil {
			return nil, errors.New("could not convert image to depth map")
		}
		w, h := dm.Width(), dm.Height()
		o.dm = dm

		obstaclePoints := make([]image.Point, 0, w*h/sampleN)
		for i := 0; i < w; i += sampleN {
			for j := 0; j < h; j++ {
				candidate := image.Pt(i, j)
				// for every sub-sampled point, figure out if it is an obstacle
			obs:
				for l := 0; l < w; l += sampleN { // continue with the sub-sampling
					for m := 0; m < h; m++ {
						compareTo := image.Pt(l, m)
						if candidate == compareTo {
							continue
						}
						if o.isCompatible(candidate, compareTo) {
							obstaclePoints = append(obstaclePoints, candidate)
							break obs
						}
					}
				}
			}
		}
		o.obstaclePts = obstaclePoints

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

/*

// buildObsDepthWithIntrinsics2 will use the methodology in Manduchi et al. to find obstacle points
// before clustering and projecting those points into 3D obstacles.
func (o *obsDepth) buildObsDepthWithIntrinsics2() segmentation.Segmenter {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		// Check if we have intrinsics here or in the camera properties. If not, don't even try
		props, err := src.Properties(ctx)
		if err != nil {
			return nil, err
		}
		if props.IntrinsicParams != nil {
			o.intrinsics = props.IntrinsicParams
		}
		if o.intrinsics == nil {
			return nil, errors.New("tried to build obstacles depth with intrinsics but no instrinsics found")
		}
		if o.depthStream == nil {
			depthStream, err := src.Stream(ctx)
			if err != nil {
				return nil, errors.Errorf("could not get stream from %s", src)
			}
			o.depthStream = depthStream
		}

		pic, release, err := o.depthStream.Next(ctx)
		if err != nil {
			return nil, errors.Errorf("could not get image from stream %s", o.depthStream)
		}
		defer release()
		dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
		if err != nil {
			return nil, errors.New("could not convert image to depth map")
		}
		w, h := dm.Width(), dm.Height()
		o.dm = dm

		obstaclePointChan := make(chan image.Point)

		start := time.Now()
		var wg sync.WaitGroup
		for i := 0; i < w; i += sampleN {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				for j := 0; j < h; j++ {
					candidate := image.Pt(i, j)
				obs:
					for l := 0; l < w; l += sampleN { // continue with the sub-sampling
						for m := 0; m < h; m++ {
							compareTo := image.Pt(l, m)
							if candidate == compareTo {
								continue
							}
							if o.isCompatible(candidate, compareTo) {
								obstaclePointChan <- candidate
								// obstaclePoints = append(obstaclePoints, candidate)
								break obs
							}
						}

					}

				}

			}(i)
		}
		go func() {
			wg.Wait()
			close(obstaclePointChan)
		}()

		mid := time.Now()
		fmt.Printf("Putting them in took %v seconds\n", mid.Sub(start))
		var d clusters.Observations

		for pt := range obstaclePointChan {
			d = append(d, clusters.Coordinates{float64(pt.X), float64(pt.Y)})
		}

		km := kmeans.New()
		outClusters, err := km.Partition(d, o.k)

		// Cluster on the 2D depth points and then project the 2D clusters into 3D boxes
		//outClusters, err := o.performKMeans(o.k)
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


// makePointList will grab points from the depthmap. Sample every n.
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

// splitPtList will split the pointlist into a list of lists, len(chunk) = chunkSize (not the last one).
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


// isObstaclePoint returns true/false if compatible with another point in the depthmap.
// as defined by Manduchi et al.
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

*/

// isCompatible will check compatibility between 2 points.
// as defined by Manduchi et al.
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

// clustersToBoxes will turn the clusters we get from kmeans into boxes.
func (o *obsDepth) clustersToBoxes(clusters clusters.Clusters) ([]spatialmath.Geometry, error) {
	boxes := make([]spatialmath.Geometry, 0, len(clusters))

	for i, c := range clusters {
		xmax, ymax, zmax := math.Inf(-1), math.Inf(-1), math.Inf(-1)
		xmin, ymin, zmin := math.Inf(1), math.Inf(1), math.Inf(1)

		for _, pt := range c.Observations {
			u, v := pt.Coordinates().Coordinates()[0], pt.Coordinates().Coordinates()[1]
			x, y, z := o.intrinsics.PixelToPoint(u, v, float64(o.dm.GetDepth(int(u), int(v))))

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

// performKMeans will do k-means clustering on all the 2D obstacle points.
func (o *obsDepth) performKMeans(k int) (clusters.Clusters, error) {
	var d clusters.Observations
	for _, pt := range o.obstaclePts {
		d = append(d, clusters.Coordinates{float64(pt.X), float64(pt.Y)})
	}
	km := kmeans.New()
	return km.Partition(d, k)
}
