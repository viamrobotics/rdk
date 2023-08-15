package segmentation

import (
	"context"
	"math"

	"github.com/golang/geo/r3"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

// ErCCLConfig specifies the necessary parameters to apply the
// connected components based clustering algo.
type ErCCLConfig struct {
	resource.TriviallyValidateConfig
	MinPtsInPlane    int       `json:"min_points_in_plane"`
	MinPtsInSegment  int       `json:"min_points_in_segment"`
	MaxDistFromPlane float64   `json:"max_dist_from_plane_mm"`
	NormalVec        r3.Vector `json:"ground_plane_normal_vec"`
	AngleTolerance   float64   `json:"ground_angle_tolerance_degs"`
	ClusteringRadius int       `json:"clustering_radius"`
	Beta             float64   `json:"beta"`
}

type node struct {
	i, j                 int
	label                int
	minHeight, maxHeight float64
	// could be implemented without i,j
	// label -1 means no cluster, otherwise labeled according to index
}

// CheckValid checks to see in the input values are valid.
func (erCCL *ErCCLConfig) CheckValid() error {
	if erCCL.MinPtsInPlane <= 0 {
		return errors.Errorf("min_points_in_plane must be greater than 0, got %v", erCCL.MinPtsInPlane)
	}
	if erCCL.MinPtsInSegment <= 0 {
		return errors.Errorf("min_points_in_segment must be greater than 0, got %v", erCCL.MinPtsInSegment)
	}
	if erCCL.MaxDistFromPlane == 0 {
		erCCL.MaxDistFromPlane = 100
	}
	if erCCL.MaxDistFromPlane <= 0 {
		return errors.Errorf("max_dist_from_plane must be greater than 0, got %v", erCCL.MaxDistFromPlane)
	}
	if erCCL.AngleTolerance > 180 || erCCL.AngleTolerance < 0 {
		return errors.Errorf("max_angle_of_plane must between 0 & 180 (inclusive), got %v", erCCL.AngleTolerance)
	}
	if erCCL.ClusteringRadius < 0 {
		return errors.Errorf("radius must be greater than 0, got %v", erCCL.ClusteringRadius)
	}
	if erCCL.Beta == 0 {
		erCCL.Beta = 5
	}
	// going to have to add that the ground plane's normal vec has to be {0, 1, 0} or {0, 0, 1}
	if !erCCL.NormalVec.IsUnit() {
		return errors.Errorf("ground_plane_normal_vec should be a unit vector, got %v", erCCL.NormalVec)
	}
	if erCCL.NormalVec.Norm2() == 0 {
		erCCL.NormalVec = r3.Vector{X: 0, Y: 0, Z: 1}
	}
	return nil
}

// ConvertAttributes changes the AttributeMap input into an ErCCLConfig.
func (erCCL *ErCCLConfig) ConvertAttributes(am utils.AttributeMap) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: erCCL})
	if err != nil {
		return err
	}
	err = decoder.Decode(am)
	if err == nil {
		err = erCCL.CheckValid()
	}
	return err
}

// NewERCCLClustering returns a Segmenter that removes the ground plane and returns a segmentation
// of the objects in a point cloud using a connected components clustering algo described in the paper
// "A Fast Spatial Clustering Method for Sparse LiDAR Point Clouds Using GPU Programming" by Tian et al. 2020.
func NewERCCLClustering(params utils.AttributeMap) (Segmenter, error) {
	// convert attributes to appropriate struct
	if params == nil {
		return nil, errors.New("config for ER-CCL segmentation cannot be nil")
	}
	cfg := &ErCCLConfig{}
	err := cfg.ConvertAttributes(params)
	if err != nil {
		return nil, err
	}
	return cfg.ErCCLAlgorithm, nil
}

// ErCCLAlgorithm applies the connected components clustering algorithm directly on a given point cloud.
func (erCCL *ErCCLConfig) ErCCLAlgorithm(ctx context.Context, src camera.VideoSource) ([]*vision.Object, error) {
	// get next point cloud
	cloud, err := src.NextPointCloud(ctx)
	if err != nil {
		return nil, err
	}
	// run ransac, get pointcloud without ground plane
	ps := NewPointCloudGroundPlaneSegmentation(cloud, erCCL.MaxDistFromPlane, erCCL.MinPtsInPlane, erCCL.AngleTolerance, erCCL.NormalVec)
	// if there are found planes, remove them, and keep all the non-plane points
	_, nonPlane, err := ps.FindGroundPlane(ctx)
	if err != nil {
		return nil, err
	}

	// need to figure out coordinate system
	// if height is not y, then height is going to be z
	heightIsY := erCCL.NormalVec.Y != 0

	// calculating s value, want 200 x 200 graph
	resolution := math.Ceil((nonPlane.MetaData().MaxX - nonPlane.MetaData().MinX) / 200)
	if heightIsY {
		resolution = math.Ceil((math.Ceil((nonPlane.MetaData().MaxZ-nonPlane.MetaData().MinZ)/200) + resolution) / 2)
	} else {
		resolution = math.Ceil((math.Ceil((nonPlane.MetaData().MaxY-nonPlane.MetaData().MinY)/200) + resolution) / 2)
	}

	// create obstacle flag map, return that 2d slice of nodes
	labelMap := pcProjection(nonPlane, resolution, heightIsY)

	// actually run erCCLL
	// iterate through every box, searching down and right r distance
	// run calculations to meet similarity threshold
	// if similar enough update to initial label value (will also be smallest)
	// iterate through pointcloud

	i := 0
	continueRunning := true
	for continueRunning {
		// 0.9 is alpha
		continueRunning := labelMapUpdate(labelMap, erCCL.ClusteringRadius, 0.9, erCCL.Beta, resolution)
		if !continueRunning {
			break
		}

		if i > 300000 { // arbitrary cutoff for iterations
			return nil, errors.New("could not converge, change parameters")
		}
		i++
	}

	// look up label value of point by looking at 2d array and seeing what label inside that struct
	// set this label
	segments := NewSegments()
	nonPlane.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
		i := int(math.Ceil((p.X - nonPlane.MetaData().MinX) / resolution))
		j := int(math.Ceil((p.Z - nonPlane.MetaData().MinZ) / resolution))
		if !heightIsY {
			j = int(math.Ceil((p.Y - nonPlane.MetaData().MinY) / resolution))
		}
		// fmt.Println("i:", i, ", j:", j)
		err := segments.AssignCluster(p, d, labelMap[i][j].label)
		if err != nil {
			panic("clustering went wrong uhhhh")
		}
		return true
	})
	// prune smaller clusters
	validClouds := pc.PrunePointClouds(segments.PointClouds(), erCCL.MinPtsInSegment)
	// wrap
	objects, err := NewSegmentsFromSlice(validClouds, "")
	if err != nil {
		return nil, err
	}
	return objects.Objects, nil
	// this seems a bit wasteful to make segments then make more segments after filtering, but rolling with it for now
}

func pcProjection(cloud pc.PointCloud, s float64, heightIsY bool) [][]node {
	h := int(math.Ceil((cloud.MetaData().MaxX-cloud.MetaData().MinX)/s)) + 1
	w := int(math.Ceil((cloud.MetaData().MaxZ-cloud.MetaData().MinZ)/s)) + 1
	if !heightIsY {
		w = int(math.Ceil((cloud.MetaData().MaxY-cloud.MetaData().MinY)/s)) + 1
	}
	retVal := make([][]node, h)
	for i := range retVal {
		retVal[i] = make([]node, w)
		for j, curNode := range retVal[i] {
			curNode.label = -1
			curNode.minHeight = 0
			curNode.maxHeight = 0
			curNode.i = i
			curNode.j = j
			retVal[i][j] = curNode
		}
	}
	cloud.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
		i := int(math.Ceil((p.X - cloud.MetaData().MinX) / s))
		j := int(math.Ceil((p.Z - cloud.MetaData().MinZ) / s))
		var curNode node
		if heightIsY {
			curNode = retVal[i][j]
			curNode.maxHeight = math.Max(curNode.maxHeight, p.Y)
			curNode.minHeight = math.Min(curNode.minHeight, p.Y)
		} else {
			j = int(math.Ceil((p.Y - cloud.MetaData().MinY) / s))
			curNode = retVal[i][j]
			curNode.maxHeight = math.Max(curNode.maxHeight, p.Z)
			curNode.minHeight = math.Min(curNode.minHeight, p.Z)
		}
		curNode.label = i*w + j
		retVal[i][j] = curNode
		return true
	})
	return retVal
}

func labelMapUpdate(labelMap [][]node, r int, alpha, beta, s float64) bool {
	mapChanged := false

	for i, curNodeSlice := range labelMap {
		for j, curNode := range curNodeSlice {
			if curNode.label == -1 {
				// skip if no points at cell
				continue
			}
			minLabel := curNode.label
			neighbors := make([]node, 0)
			// finding neighbors + finding min label value
			for x := 0; x < r; x++ {
				newI := i + x
				if newI >= len(labelMap) {
					break
				}
				for y := 0; y < r; y++ {
					newJ := j + y
					if newJ >= len(curNodeSlice) {
						break
					}
					if newI == i && newJ == j {
						continue // might be able to remove this because original should be in neighbors list
					}
					neighborNode := labelMap[newI][newJ]
					if similarEnough(curNode, neighborNode, r, alpha, beta, s) {
						neighbors = append(neighbors, neighborNode)
						minLabel = int(math.Min(float64(minLabel), float64(neighborNode.label)))
					}
				}
			}
			if minLabel != curNode.label {
				mapChanged = true
				labelMap[curNode.i][curNode.j].label = minLabel
			}
			for _, neighbor := range neighbors {
				if neighbor.label != minLabel {
					mapChanged = true
					labelMap[neighbor.i][neighbor.j].label = minLabel
				}
			}
		}
	}
	return mapChanged
}

func similarEnough(curNode, neighbor node, r int, alpha, beta, s float64) bool {
	// trying to avoid math.pow since these are ints and math.pow is slow
	if neighbor.label == -1 {
		return false
	}
	d := s * math.Sqrt(float64(((curNode.i-neighbor.i)*(curNode.i-neighbor.i) + (curNode.j-neighbor.j)*(curNode.j-neighbor.j))))
	h := math.Abs(curNode.maxHeight-neighbor.maxHeight) + math.Abs(curNode.minHeight-neighbor.minHeight)
	ecc := alpha*math.Exp(-d) + (1-alpha)*math.Exp(-h)
	return ecc >= beta*math.Exp(float64(-r))
}
