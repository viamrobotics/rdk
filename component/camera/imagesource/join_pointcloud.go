package imagesource

import (
	"context"
	"fmt"
	"image"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"join_pointclouds",
		registry.Component{RobotConstructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*JoinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newJoinPointCloudSource(ctx, r, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "join_pointclouds",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf JoinAttrs
			attrs, err := config.TransformAttributeMapToStruct(&conf, attributes)
			if err != nil {
				return nil, err
			}
			result, ok := attrs.(*JoinAttrs)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(result, attrs)
			}
			return result, nil
		},
		&JoinAttrs{})
}

// JoinAttrs is the attribute struct for joinPointCloudSource.
type JoinAttrs struct {
	*camera.AttrConfig
	TargetFrame   string   `json:"target_frame"`
	SourceCameras []string `json:"source_cameras"`
	MergeMethod   string   `json:"merge_method"`
}

type (
	// MergeMethodType Defines which strategy is used for merging.
	MergeMethodType string
	// MergeMethodUnsupportedError is returned when the merge method is not supported.
	MergeMethodUnsupportedError error
)

const (
	// Null is a default value for the merge method.
	Null MergeMethodType = ""
	// Naive is the naive merge method.
	Naive MergeMethodType = "naive"
	// ICP is the ICP merge method.
	ICP MergeMethodType = "icp"
)

func readMergeMethodType(s string) (MergeMethodType, error) {
	switch s {
	case "naive", "":
		return Naive, nil
	case "icp":
		return ICP, nil
	default:
		return Null, newMergeMethodUnsupportedError(s)
	}
}

func newMergeMethodUnsupportedError(method string) MergeMethodUnsupportedError {
	return errors.Errorf("merge method %s not supported", method)
}

// joinPointCloudSource takes image sources that can produce point clouds and merges them together from
// the point of view of targetName. The model needs to have the entire robot available in order to build the correct offsets
// between robot components for the frame system transform.
type joinPointCloudSource struct {
	generic.Unimplemented
	sourceCameras []camera.Camera
	sourceNames   []string
	targetName    string
	robot         robot.Robot
	stream        camera.StreamType
	mergeMethod   MergeMethodType
}

// newJoinPointCloudSource creates a camera that combines point cloud sources into one point cloud in the
// reference frame of targetName.
func newJoinPointCloudSource(ctx context.Context, r robot.Robot, attrs *JoinAttrs) (camera.Camera, error) {
	joinSource := &joinPointCloudSource{}
	// frame to merge from
	joinSource.sourceCameras = make([]camera.Camera, len(attrs.SourceCameras))
	joinSource.sourceNames = make([]string, len(attrs.SourceCameras))
	for i, source := range attrs.SourceCameras {
		joinSource.sourceNames[i] = source
		camSource, err := camera.FromRobot(r, source)
		if err != nil {
			return nil, fmt.Errorf("no camera source called (%s): %w", source, err)
		}
		joinSource.sourceCameras[i] = camSource
	}
	// frame to merge to
	joinSource.targetName = attrs.TargetFrame
	joinSource.robot = r
	joinSource.stream = camera.StreamType(attrs.Stream)

	mergeMethod, err := readMergeMethodType(attrs.MergeMethod)
	if err != nil {
		return nil, fmt.Errorf("invalid merge method (%s): %w", attrs.MergeMethod, err)
	}
	joinSource.mergeMethod = mergeMethod

	if idx, ok := contains(joinSource.sourceNames, joinSource.targetName); ok {
		proj, _ := camera.GetProjector(ctx, nil, joinSource.sourceCameras[idx])
		return camera.New(joinSource, proj)
	}
	return camera.New(joinSource, nil)
}

// NextPointCloud gets all the point clouds from the source cameras,
// and puts the points in one point cloud in the frame of targetFrame.
func (jpcs *joinPointCloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	switch jpcs.mergeMethod {
	case Naive:
		return jpcs.NextPointCloudNaive(ctx)
	case ICP:
		return jpcs.nextPointCloudICP(ctx)
	case Null:
		return jpcs.NextPointCloudNaive(ctx)
	default:
		return nil, newMergeMethodUnsupportedError(string(jpcs.mergeMethod))
	}
}

func (jpcs *joinPointCloudSource) NextPointCloudNaive(ctx context.Context) (pointcloud.PointCloud, error) {
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloud")
	defer span.End()

	fs, err := framesystem.RobotFrameSystem(ctx, jpcs.robot, nil)
	if err != nil {
		return nil, err
	}

	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}

	finalPoints := make(chan []pointcloud.PointAndData, 50)
	activeReaders := int32(len(jpcs.sourceCameras))

	for i, cam := range jpcs.sourceCameras {
		iCopy := i
		camCopy := cam
		utils.PanicCapturingGo(func() {
			ctx, span := trace.StartSpan(ctx, "camera::joinPointCloudSource::NextPointCloud::"+jpcs.sourceNames[iCopy])
			defer span.End()

			defer func() {
				atomic.AddInt32(&activeReaders, -1)
			}()
			pcSrc, err := func() (pointcloud.PointCloud, error) {
				ctx, span := trace.StartSpan(ctx, "camera::joinPointCloudSource::NextPointCloud::"+jpcs.sourceNames[iCopy]+"-NextPointCloud")
				defer span.End()
				return camCopy.NextPointCloud(ctx)
			}()
			if err != nil {
				panic(err) // TODO(erh) is there something better to do?
			}

			sourceFrame := referenceframe.NewPoseInFrame(jpcs.sourceNames[iCopy], spatialmath.NewZeroPose())
			theTransform, err := fs.Transform(inputs, sourceFrame, jpcs.targetName)
			if err != nil {
				panic(err) // TODO(erh) is there something better to do?
			}

			var wg sync.WaitGroup
			const numLoops = 8
			wg.Add(numLoops)
			for loop := 0; loop < numLoops; loop++ {
				f := func(loop int) {
					defer wg.Done()
					const batchSize = 500
					batch := make([]pointcloud.PointAndData, 0, batchSize)
					savedDualQuat := spatialmath.NewZeroPose()
					pcSrc.Iterate(numLoops, loop, func(p r3.Vector, d pointcloud.Data) bool {
						if jpcs.sourceNames[iCopy] != jpcs.targetName {
							spatialmath.ResetPoseDQTransalation(savedDualQuat, p)
							newPose := spatialmath.Compose(theTransform.(*referenceframe.PoseInFrame).Pose(), savedDualQuat)
							p = newPose.Point()
						}
						batch = append(batch, pointcloud.PointAndData{P: p, D: d})
						if len(batch) > batchSize {
							finalPoints <- batch
							batch = make([]pointcloud.PointAndData, 0, batchSize)
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

	var pcTo pointcloud.PointCloud

	dataLastTime := false
	for dataLastTime || atomic.LoadInt32(&activeReaders) > 0 {
		select {
		case ps := <-finalPoints:
			for _, p := range ps {
				if pcTo == nil {
					if p.D == nil {
						pcTo = pointcloud.NewAppendOnlyOnlyPointsPointCloud(len(jpcs.sourceNames) * 640 * 800)
					} else {
						pcTo = pointcloud.NewWithPrealloc(len(jpcs.sourceNames) * 640 * 800)
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

type icpMergeResultInfo struct {
	x0        []float64
	optResult optimize.Result
}

func (jpcs *joinPointCloudSource) mergePointCloudsICP(ctx context.Context, sourceIndex int, fs *referenceframe.FrameSystem,
	inputs *map[string][]referenceframe.Input, target *pointcloud.KDTree,
) (pointcloud.PointCloud, icpMergeResultInfo, error) {
	// This function registers a point cloud to the reference frame of a target point cloud.
	// This is accomplished using ICP (Iterative Closest Point) to align the two point clouds.
	// The loss function being used is the average distance between corresponding points in the registered point clouds.
	// The optimization is performed using BFGS (Broyden-Fletcher-Goldfarb-Shanno)
	// optimization on parameters representing a transformation matrix.

	// lint:ignore This is just used as a stop gap while waiting on another PR.
	debug := false // In a future PR (when jpcs is a camera) this will be done with a param.
	pcSrc, err := jpcs.sourceCameras[sourceIndex].NextPointCloud(ctx)
	if err != nil {
		return nil, icpMergeResultInfo{}, err
	}

	sourcePointList := make([]r3.Vector, pcSrc.Size())
	nearestNeighborBuffer := make([]r3.Vector, pcSrc.Size())

	var nearest r3.Vector
	var index int
	pcSrc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		sourcePointList[index] = p
		nearest, _, _, _ = target.NearestNeighbor(p)
		nearestNeighborBuffer[index] = nearest
		index++
		return true
	})
	// create optimization problem
	optFunc := func(x []float64) float64 {
		// x is an 6-vector used to create a pose
		point := r3.Vector{X: x[0], Y: x[1], Z: x[2]}
		orient := spatialmath.EulerAngles{Roll: x[3], Pitch: x[4], Yaw: x[5]}

		pose := spatialmath.NewPoseFromOrientation(point, &orient)

		// compute the error
		var dist float64
		var currPose spatialmath.Pose
		// TODO parallelize this
		for i := 0; i < len(sourcePointList); i++ {
			currPose = spatialmath.NewPoseFromPoint(sourcePointList[i])
			transformedP := spatialmath.Compose(pose, currPose).Point()
			nearest := nearestNeighborBuffer[i]
			nearestNeighborBuffer[i], _, _, _ = target.NearestNeighbor(transformedP)
			dist += math.Sqrt(math.Pow((transformedP.X-nearest.X), 2) +
				math.Pow((transformedP.Y-nearest.Y), 2) +
				math.Pow((transformedP.Z-nearest.Z), 2))
		}

		return dist / float64(pcSrc.Size())
	}
	grad := func(grad, x []float64) {
		fd.Gradient(grad, optFunc, x, nil)
	}
	hess := func(h *mat.SymDense, x []float64) {
		fd.Hessian(h, optFunc, x, nil)
	}

	sourceFrame := referenceframe.NewPoseInFrame(jpcs.sourceNames[sourceIndex], spatialmath.NewZeroPose())
	theTransform, err := (*fs).Transform(*inputs, sourceFrame, jpcs.targetName)
	if err != nil {
		return nil, icpMergeResultInfo{}, err
	}
	x0 := make([]float64, 6)
	x0[0] = theTransform.(*referenceframe.PoseInFrame).Pose().Point().X
	x0[1] = theTransform.(*referenceframe.PoseInFrame).Pose().Point().Y
	x0[2] = theTransform.(*referenceframe.PoseInFrame).Pose().Point().Z
	x0[3] = theTransform.(*referenceframe.PoseInFrame).Pose().Orientation().EulerAngles().Roll
	x0[4] = theTransform.(*referenceframe.PoseInFrame).Pose().Orientation().EulerAngles().Pitch
	x0[5] = theTransform.(*referenceframe.PoseInFrame).Pose().Orientation().EulerAngles().Yaw

	if debug {
		utils.Logger.Debugf("x0 = %v", x0)
	}

	prob := optimize.Problem{Func: optFunc, Grad: grad, Hess: hess}

	// setup optimizer
	settings := &optimize.Settings{
		GradientThreshold: 0,
		Converger: &optimize.FunctionConverge{
			Relative:   1e-10,
			Absolute:   1e-10,
			Iterations: 100,
		},
		MajorIterations: 100,
		// Recorder:        optimize.NewPrinter(),
	}

	method := &optimize.BFGS{
		Linesearcher: &optimize.Bisection{
			CurvatureFactor: 0.9,
		},
	}

	// run optimization
	res, err := optimize.Minimize(prob, x0, settings, method)
	if debug {
		utils.Logger.Debugf("res = %v, err = %v", res, err)
	}

	x := res.Location.X

	// create the new pose
	point := r3.Vector{X: x[0], Y: x[1], Z: x[2]}
	orient := spatialmath.EulerAngles{Roll: x[3], Pitch: x[4], Yaw: x[5]}
	pose := spatialmath.NewPoseFromOrientation(point, &orient)

	// transform the pointcloud
	registeredPointCloud := pointcloud.NewWithPrealloc(pcSrc.Size())
	pcSrc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		posePoint := spatialmath.NewPoseFromPoint(p)
		transformedP := spatialmath.Compose(pose, posePoint).Point()
		err := registeredPointCloud.Set(transformedP, d)
		return err == nil
	})

	return registeredPointCloud, icpMergeResultInfo{x0: x0, optResult: *res}, nil
}

func (jpcs *joinPointCloudSource) nextPointCloudICP(ctx context.Context) (pointcloud.PointCloud, error) {
	// lint:ignore This is just used as a stop gap while waiting on another PR.
	debug := false // In a future PR (when jpcs is a camera) this will be done with a param.
	ctx, span := trace.StartSpan(ctx, "joinPointCloudSource::NextPointCloud")
	defer span.End()

	fs, err := framesystem.RobotFrameSystem(ctx, jpcs.robot, nil)
	if err != nil {
		return nil, err
	}

	inputs, err := jpcs.initializeInputs(ctx, fs)
	if err != nil {
		return nil, err
	}

	var targetIndex int

	for i, camName := range jpcs.sourceNames {
		if camName == jpcs.targetName {
			targetIndex = i
			break
		}
	}

	targetPointCloud, err := jpcs.sourceCameras[targetIndex].NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}

	finalPointCloud := pointcloud.NewKDTree(targetPointCloud)
	for i := range jpcs.sourceCameras {
		if i == targetIndex {
			continue
		}

		registeredPointCloud, info, err := jpcs.mergePointCloudsICP(ctx, i, &fs, &inputs, finalPointCloud)
		if err != nil {
			panic(err) // TODO(erh) is there something better to do?
		}
		if debug {
			utils.Logger.Debugf("Learned Transform = %v", info.optResult.Location.X)
		}
		transformDist := math.Sqrt(math.Pow(info.optResult.Location.X[0]-info.x0[0], 2) +
			math.Pow(info.optResult.Location.X[1]-info.x0[1], 2) +
			math.Pow(info.optResult.Location.X[2]-info.x0[2], 2))
		if transformDist > 100 {
			utils.Logger.Warnf(`Transform is %f away from transform defined in frame system. 
			This may indicate an incorrect frame system.`, transformDist)
		}
		// TODO(aidanglickman) this loop is highly parallelizable, not yet making use
		registeredPointCloud.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
			nearest, _, _, _ := finalPointCloud.NearestNeighbor(p)
			distance := math.Sqrt(math.Pow(p.X-nearest.X, 2) + math.Pow(p.Y-nearest.Y, 2) + math.Pow(p.Z-nearest.Z, 2))
			if distance > 1e-2 { // TODO This should probably be a param. Value is highly dependent on the size and accuracy of given pointclouds.
				err = finalPointCloud.Set(p, d)
				if err != nil {
					return false
				}
			}
			return true
		})
	}

	return finalPointCloud, nil
}

// initalizeInputs gets all the input positions for the robot components in order to calculate the frame system offsets.
func (jpcs *joinPointCloudSource) initializeInputs(
	ctx context.Context,
	fs referenceframe.FrameSystem,
) (map[string][]referenceframe.Input, error) {
	inputs := referenceframe.StartPositions(fs)

	for k, original := range inputs {
		if strings.HasSuffix(k, "_offset") {
			continue
		}
		if len(original) == 0 {
			continue
		}

		all := robot.AllResourcesByName(jpcs.robot, k)
		if len(all) != 1 {
			return nil, fmt.Errorf("got %d resources instead of 1 for (%s)", len(all), k)
		}

		ii, ok := all[0].(referenceframe.InputEnabled)
		if !ok {
			return nil, fmt.Errorf("%v(%T) is not InputEnabled", k, all[0])
		}

		pos, err := ii.CurrentInputs(ctx)
		if err != nil {
			return nil, err
		}
		inputs[k] = pos
	}
	return inputs, nil
}

// Next gets the merged point cloud from all sources, and then uses a projection to turn it into a 2D image.
func (jpcs *joinPointCloudSource) Next(ctx context.Context) (image.Image, func(), error) {
	var proj rimage.Projector
	var err error
	if idx, ok := contains(jpcs.sourceNames, jpcs.targetName); ok {
		proj, err = jpcs.sourceCameras[idx].GetProperties(ctx)
		if err != nil && !errors.Is(err, transform.ErrNoIntrinsics) {
			return nil, nil, err
		}
	}
	if proj == nil { // use a default projector if target frame doesn't have one
		proj = &rimage.ParallelProjection{}
	}

	pc, err := jpcs.NextPointCloud(ctx)
	if err != nil {
		return nil, nil, err
	}
	img, dm, err := proj.PointCloudToRGBD(pc)
	if err != nil {
		return nil, nil, err
	}
	switch jpcs.stream {
	case camera.UnspecifiedStream, camera.ColorStream, camera.BothStream:
		return img, func() {}, nil
	case camera.DepthStream:
		return dm, func() {}, nil
	default:
		return nil, nil, camera.NewUnsupportedStreamError(jpcs.stream)
	}
}

func contains(s []string, str string) (int, bool) {
	for i, v := range s {
		if v == str {
			return i, true
		}
	}
	return -1, false
}
