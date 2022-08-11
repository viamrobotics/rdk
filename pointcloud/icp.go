package pointcloud

import (
	"math"
	"sync"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize"

	"go.viam.com/rdk/spatialmath"
)

// IcpMergeResultInfo is a struct to hold the results of registering a pointcloud.
type IcpMergeResultInfo struct {
	X0        []float64
	OptResult optimize.Result
}

// RegisterPointCloudICP registers a source pointcloud to a target pointcloud, starting from an initial guess using ICP.
func RegisterPointCloudICP(pcSrc PointCloud, target *KDTree, guess spatialmath.Pose, debug bool, numThreads int,
) (PointCloud, IcpMergeResultInfo, error) {
	// This function registers a point cloud to the reference frame of a target point cloud.
	// This is accomplished using ICP (Iterative Closest Point) to align the two point clouds.
	// The loss function being used is the average distance between corresponding points in the registered point clouds.
	// The optimization is performed using BFGS (Broyden-Fletcher-Goldfarb-Shanno)
	// optimization on parameters representing a transformation matrix.

	sourcePointList := make([]r3.Vector, pcSrc.Size())
	nearestNeighborBuffer := make([]r3.Vector, pcSrc.Size())

	var initWg sync.WaitGroup
	initWg.Add(numThreads)
	batchSize := (pcSrc.Size() + numThreads - 1) / numThreads // Round up to avoid missing indices
	for i := 0; i < numThreads; i++ {
		f := func(thread int) {
			defer initWg.Done()
			offset := 0
			pcSrc.Iterate(numThreads, thread, func(p r3.Vector, d Data) bool {
				index := thread*batchSize + offset
				if index >= pcSrc.Size() {
					return false
				}
				sourcePointList[index] = p
				nearest, _, _, _ := target.NearestNeighbor(p)
				nearestNeighborBuffer[index] = nearest
				offset++
				return true
			})
		}
		iCopy := i
		utils.PanicCapturingGo(func() { f(iCopy) })
	}
	initWg.Wait()

	// create optimization problem
	optFunc := func(x []float64) float64 {
		// x is an 6-vector used to create a pose
		point := r3.Vector{X: x[0], Y: x[1], Z: x[2]}
		orient := spatialmath.EulerAngles{Roll: x[3], Pitch: x[4], Yaw: x[5]}

		pose := spatialmath.NewPoseFromOrientation(point, &orient)
		// compute the error
		distChan := make(chan float64, numThreads)
		var optWg sync.WaitGroup
		optWg.Add(numThreads)
		optBatchSize := (len(sourcePointList) + numThreads - 1) / numThreads
		for thread := 0; thread < numThreads; thread++ {
			distBuf := 0.
			optFunc := func(thread int) {
				defer optWg.Done()
				for offset := 0; offset < optBatchSize; offset++ {
					i := thread*optBatchSize + offset
					if i >= len(sourcePointList) {
						break
					}
					currPose := spatialmath.NewPoseFromPoint(sourcePointList[i])
					transformedP := spatialmath.Compose(pose, currPose).Point()
					nearest := nearestNeighborBuffer[i]
					nearestNeighborBuffer[i], _, _, _ = target.NearestNeighbor(transformedP)
					distBuf += math.Sqrt(math.Pow((transformedP.X-nearest.X), 2) +
						math.Pow((transformedP.Y-nearest.Y), 2) +
						math.Pow((transformedP.Z-nearest.Z), 2))
				}
				distChan <- distBuf
			}
			threadCopy := thread
			utils.PanicCapturingGo(func() { optFunc(threadCopy) })
		}
		optWg.Wait()

		dist := 0.0
		for i := 0; i < numThreads; i++ {
			dist += <-distChan
		}
		return dist / float64(pcSrc.Size())
	}
	grad := func(grad, x []float64) {
		fd.Gradient(grad, optFunc, x, nil)
	}
	hess := func(h *mat.SymDense, x []float64) {
		fd.Hessian(h, optFunc, x, nil)
	}

	x0 := make([]float64, 6)
	x0[0] = guess.Point().X
	x0[1] = guess.Point().Y
	x0[2] = guess.Point().Z
	x0[3] = guess.Orientation().EulerAngles().Roll
	x0[4] = guess.Orientation().EulerAngles().Pitch
	x0[5] = guess.Orientation().EulerAngles().Yaw

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
	registeredPointCloud := NewWithPrealloc(pcSrc.Size())

	pcSrc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		posePoint := spatialmath.NewPoseFromPoint(p)
		transformedP := spatialmath.Compose(pose, posePoint).Point()
		err = registeredPointCloud.Set(transformedP, d)
		return err == nil
	})

	return registeredPointCloud, IcpMergeResultInfo{X0: x0, OptResult: *res}, nil
}
