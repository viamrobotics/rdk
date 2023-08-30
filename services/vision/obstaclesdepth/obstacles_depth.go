// Package obstaclesdepth uses an underlying depth camera to fulfill GetObjectPointClouds,
// using the method outlined in (Manduchi, Roberto, et al. "Obstacle detection and terrain classification
// for autonomous off-road navigation." Autonomous robots 18 (2005): 81-102.)
package obstaclesdepth

import (
	"context"
	"image"
	"math"
	"sort"
	"strconv"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"
	"github.com/pkg/errors"
	"github.com/viamrobotics/gostream"
	"go.opencensus.io/trace"
	goutils "go.viam.com/utils"

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
)

var model = resource.DefaultModelFamily.WithModel("obstacles_depth")

// ObsDepthConfig specifies the parameters to be used for the obstacle depth service.
type ObsDepthConfig struct {
	Hmin           float64 `json:"h_min_m"`
	Hmax           float64 `json:"h_max_m"`
	ThetaMax       float64 `json:"theta_max_deg"`
	ReturnPCDs     bool    `json:"return_pcds"`
	WithGeometries *bool   `json:"with_geometries"`
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
	withGeoms   bool
	k           int
	depthStream gostream.VideoStream
}

const (
	// the first 3 consts are parameters from Manduchi et al.
	defaultHmin     = 0.0
	defaultHmax     = 1.0
	defaultThetamax = 45

	defaultK = 10 // default number of obstacle segments to create
	sampleN  = 4  // we sample 1 in every sampleN depth points
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
	if config.Hmin >= config.Hmax && !(config.Hmin == 0 && config.Hmax == 0) {
		return nil, errors.New("Hmin should be less than Hmax")
	}
	if config.Hmin < 0 {
		return nil, errors.New("Hmin should be greater than or equal to 0")
	}
	if config.Hmax < 0 {
		return nil, errors.New("Hmax should be greater than or equal to 0")
	}
	if config.ThetaMax < 0 || config.ThetaMax > 360 {
		return nil, errors.New("ThetaMax should be in degrees between 0 and 360")
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
	_, span := trace.StartSpan(ctx, "service::vision::registerObstacleDepth")
	defer span.End()
	if conf == nil {
		return nil, errors.New("config for obstacles_depth cannot be nil")
	}

	// Use defaults if needed
	if conf.Hmax == 0 {
		conf.Hmax = defaultHmax
	}
	if conf.ThetaMax == 0 {
		conf.ThetaMax = defaultThetamax
	}
	if conf.WithGeometries == nil {
		wg := true
		conf.WithGeometries = &wg
	}

	sinTheta := math.Sin(conf.ThetaMax * math.Pi / 180) // sin(radians(theta))
	myObsDep := obsDepth{
		hMin: 1000 * conf.Hmin, hMax: 1000 * conf.Hmax, sinTheta: sinTheta,
		returnPCDs: conf.ReturnPCDs, k: defaultK, withGeoms: *conf.WithGeometries,
	}

	segmenter := myObsDep.buildObsDepth(logger) // does the thing
	return svision.NewService(name, r, nil, nil, nil, segmenter)
}

// BuildObsDepth will check for intrinsics and determine how to build based on that.
func (o *obsDepth) buildObsDepth(logger golog.Logger) func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	return func(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
		props, err := src.Properties(ctx)
		if err != nil {
			logger.Warnw("could not find camera properties. obstacles depth started without camera's intrinsic parameters", "error", err)
			return o.obsDepthNoIntrinsics(ctx, src)
		}
		if props.IntrinsicParams == nil {
			logger.Warn("obstacles depth started but camera did not have intrinsic parameters")
			return o.obsDepthNoIntrinsics(ctx, src)
		}
		o.intrinsics = props.IntrinsicParams
		if o.withGeoms {
			return o.obsDepthWithIntrinsics(ctx, src)
		}
		return o.obsDepthNoIntrinsics(ctx, src)
	}
}

// buildObsDepthNoIntrinsics will return the median depth in the depth map as a Geometry point.
func (o *obsDepth) obsDepthNoIntrinsics(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	pic, release, err := camera.ReadImage(ctx, src)
	if err != nil {
		return nil, errors.Errorf("could not get image from %s", src)
	}
	defer release()

	dm, err := rimage.ConvertImageToDepthMap(ctx, pic)
	if err != nil {
		return nil, errors.New("could not convert image to depth map")
	}
	depData := dm.Data()
	if len(depData) == 0 {
		return nil, errors.New("could not get info from depth map")
	}
	// Sort the depth data [smallest...largest]
	sort.Slice(depData, func(i, j int) bool {
		return depData[i] < depData[j]
	})
	med := int(0.5 * float64(len(depData)))
	pt := spatialmath.NewPoint(r3.Vector{X: 0, Y: 0, Z: float64(depData[med])}, "")
	toReturn := make([]*vision.Object, 1)
	toReturn[0] = &vision.Object{Geometry: pt}
	return toReturn, nil
}

// buildObsDepthWithIntrinsics will use the methodology in Manduchi et al. to find obstacle points
// before clustering and projecting those points into 3D obstacles.
func (o *obsDepth) obsDepthWithIntrinsics(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	// Check if we have intrinsics here. If not, don't even try
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

	var wg sync.WaitGroup
	obstaclePointChan := make(chan image.Point)

	for i := 0; i < w; i += sampleN {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < h; j++ {
				candidate := image.Pt(i, j)
			obs: // for every sub-sampled point, figure out if it is an obstacle
				for l := 0; l < w; l += sampleN { // continue with the sub-sampling
					for m := 0; m < h; m++ {
						compareTo := image.Pt(l, m)
						if candidate == compareTo {
							continue
						}
						if o.isCompatible(candidate, compareTo) {
							obstaclePointChan <- candidate
							break obs
						}
					}
				}
			}
		}(i)
	}

	goutils.ManagedGo(func() {
		wg.Wait()
		close(obstaclePointChan)
	}, nil)

	obstaclePoints := make([]image.Point, 0, w*h/sampleN)
	for op := range obstaclePointChan {
		obstaclePoints = append(obstaclePoints, op)
	}
	o.obstaclePts = obstaclePoints

	// Cluster the points in 3D
	boxes, outClusters, err := o.performKMeans3D(o.k)
	if err != nil {
		return nil, err
	}

	// Packaging the return depending on if they want PCDs
	n := int(math.Min(float64(len(outClusters)), float64(len(boxes)))) // should be same len but for safety
	toReturn := make([]*vision.Object, n)
	for i := 0; i < n; i++ { // for each cluster/box make an object
		if o.returnPCDs {
			pcdToReturn := pointcloud.NewWithPrealloc(len(outClusters[i].Observations))
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

// isCompatible will check compatibility between 2 points.
// as defined by Manduchi et al.
func (o *obsDepth) isCompatible(p1, p2 image.Point) bool {
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

// performKMeans3D will do k-means clustering on projected obstacle points.
func (o *obsDepth) performKMeans3D(k int) ([]spatialmath.Geometry, clusters.Clusters, error) {
	var d clusters.Observations
	for _, pt := range o.obstaclePts {
		outX, outY, outZ := o.intrinsics.PixelToPoint(float64(pt.X), float64(pt.Y), float64(o.dm.GetDepth(pt.X, pt.Y)))
		d = append(d, clusters.Coordinates{outX, outY, outZ})
	}
	km := kmeans.New()
	clusters, err := km.Partition(d, k)
	if err != nil {
		return nil, nil, err
	}
	boxes := make([]spatialmath.Geometry, 0, len(clusters))

	for i, c := range clusters {
		xmax, ymax, zmax := math.Inf(-1), math.Inf(-1), math.Inf(-1)
		xmin, ymin, zmin := math.Inf(1), math.Inf(1), math.Inf(1)

		for _, pt := range c.Observations {
			x, y, z := pt.Coordinates().Coordinates()[0], pt.Coordinates().Coordinates()[1], pt.Coordinates().Coordinates()[2]

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
		// Make a box from those bounds and add it in
		xdiff, ydiff, zdiff := xmax-xmin, ymax-ymin, zmax-zmin
		xc, yc, zc := (xmin+xmax)/2, (ymin+ymax)/2, (zmin+zmax)/2
		pose := spatialmath.NewPose(r3.Vector{xc, yc, zc}, spatialmath.NewZeroOrientation())

		box, err := spatialmath.NewBox(pose, r3.Vector{xdiff, ydiff, zdiff}, strconv.Itoa(i))
		if err != nil {
			return nil, nil, err
		}
		boxes = append(boxes, box)
	}
	return boxes, clusters, err
}
