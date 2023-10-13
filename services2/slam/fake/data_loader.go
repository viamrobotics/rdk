package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/spatialmath"
)

const chunkSizeBytes = 1 * 1024 * 1024

type pose struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
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
	Pose               pose   `json:"pose"`
	ComponentReference string `json:"component_reference"`
	Extra              extra  `json:"extra"`
}

var maxDataCount = 24

const (
	internalStateTemplate = "%s/internal_state/internal_state_%d.pbstream"
	pcdTemplate           = "%s/pointcloud/pointcloud_%d.pcd"
	positionTemplate      = "%s/position/position_%d.json"
)

func fakePointCloudMap(_ context.Context, datasetDir string, slamSvc *SLAM) (func() ([]byte, error), error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(pcdTemplate, datasetDir, slamSvc.getCount())))
	slamSvc.logger.Debug("Reading " + path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	chunk := make([]byte, chunkSizeBytes)
	f := func() ([]byte, error) {
		bytesRead, err := file.Read(chunk)
		if err != nil {
			defer utils.UncheckedErrorFunc(file.Close)
			return nil, err
		}
		return chunk[:bytesRead], err
	}
	return f, nil
}

func fakeInternalState(_ context.Context, datasetDir string, slamSvc *SLAM) (func() ([]byte, error), error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(internalStateTemplate, datasetDir, slamSvc.getCount())))
	slamSvc.logger.Debug("Reading " + path)
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	chunk := make([]byte, chunkSizeBytes)
	f := func() ([]byte, error) {
		bytesRead, err := file.Read(chunk)
		if err != nil {
			defer utils.UncheckedErrorFunc(file.Close)
			return nil, err
		}
		return chunk[:bytesRead], err
	}
	return f, nil
}

func fakePosition(_ context.Context, datasetDir string, slamSvc *SLAM) (spatialmath.Pose, string, error) {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(positionTemplate, datasetDir, slamSvc.getCount())))
	slamSvc.logger.Debug("Reading " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}

	position, err := positionFromJSON(data)
	if err != nil {
		return nil, "", err
	}
	p := r3.Vector{X: position.Pose.X, Y: position.Pose.Y, Z: position.Pose.Z}

	quat := position.Extra.Quat
	orientation := &spatialmath.Quaternion{Real: quat.Real, Imag: quat.Imag, Jmag: quat.Jmag, Kmag: quat.Kmag}
	pose := spatialmath.NewPose(p, orientation)

	return pose, position.ComponentReference, nil
}

func positionFromJSON(data []byte) (position, error) {
	position := position{}

	if err := json.Unmarshal(data, &position); err != nil {
		return position, err
	}
	return position, nil
}
