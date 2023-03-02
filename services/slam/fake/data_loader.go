package fake

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

type pose struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type poseInFrame struct {
	Pose pose `json:"pose"`
}

type quat struct {
	Imag float64 `json:"imag"`
	Jmag float64 `json:"jmag"`
	Kmag float64 `json:"kmag"`
	Real float64 `json:"real"`
}

type extra struct {
	Quat quat `json:"quat"`
}

type position struct {
	Pose  poseInFrame `json:"pose"`
	Extra extra       `json:"extra"`
}

// type positionNew struct {
// 	Pose               pose   `json:"pose"`
// 	ComponentReference string `json:"component_reference"`
// 	Extra              extra  `json:"extra"`
// }

const (
	internalStateTemplate = "%s/internal_state/internal_state_%d.pbstream"
	maxDataCount          = 16
	pcdTemplate           = "%s/pointcloud/pointcloud_%d.pcd"
	jpegTemplate          = "%s/image_map/image_map_%d.jpeg"
	positionTemplate      = "%s/position/position_%d.json"
	// positionNewTemplate   = "%s/position_new/position_%d.json".
)

func fakeGetMap(datasetDir string, slamSvc *SLAM, mimeType string) (string, image.Image, *vision.Object, error) {
	var err error
	var img image.Image
	var vObj *vision.Object

	switch mimeType {
	case rdkutils.MimeTypePCD:
		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDir, slamSvc.getCount())))
		slamSvc.logger.Debug("Reading " + path)

		f, err := os.Open(path)
		if err != nil {
			return "", nil, nil, err
		}
		defer utils.UncheckedErrorFunc(f.Close)

		pc, err := pointcloud.ReadPCD(f)
		if err != nil {
			return "", nil, nil, err
		}

		vObj, err = vision.NewObject(pc)
		if err != nil {
			return "", nil, nil, err
		}

	case rdkutils.MimeTypeJPEG:
		path := filepath.Clean(artifact.MustPath(fmt.Sprintf(jpegTemplate, datasetDir, slamSvc.getCount())))
		slamSvc.logger.Debug("Reading " + path)
		img, err = rimage.NewImageFromFile(path)

	default:
		return "", nil, nil, errors.New("received invalid mimeType for GetMap call")
	}

	if err != nil {
		return "", nil, nil, err
	}

	return mimeType, img, vObj, nil
}

func fakeGetInternalState(datasetDir string, slamSvc *SLAM) ([]byte, error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(internalStateTemplate, datasetDir, slamSvc.getCount())))
	slamSvc.logger.Debug("Reading " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func fakePosition(datasetDir string, slamSvc *SLAM, name string) (*referenceframe.PoseInFrame, error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(positionTemplate, datasetDir, slamSvc.getCount())))
	slamSvc.logger.Debug("Reading " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	position, err := positionFromJSON(data)
	if err != nil {
		return nil, err
	}
	p := r3.Vector{X: position.Pose.Pose.X, Y: position.Pose.Pose.Y, Z: position.Pose.Pose.Z}

	quat := position.Extra.Quat
	orientation := &spatialmath.Quaternion{Real: quat.Real, Imag: quat.Imag, Jmag: quat.Jmag, Kmag: quat.Kmag}
	pose := spatialmath.NewPose(p, orientation)
	pInFrame := referenceframe.NewPoseInFrame(name, pose)

	return pInFrame, nil
}

func positionFromJSON(data []byte) (position, error) {
	position := position{}

	if err := json.Unmarshal(data, &position); err != nil {
		return position, err
	}
	return position, nil
}
