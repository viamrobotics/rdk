package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/utils"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
)

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

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewDebugLogger("robot_server"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	progName := args[0]
	if len(args) != 3 {
		return fmt.Errorf("usage: %s <source_map_file_path.pcd> <dest_file_path.jpeg>", progName)
	}

	ppRM := transform.ParallelProjectionOntoXYWithRobotMarker{}

	sourcePcdFilePath := args[1]
	pcdFile, err := os.Open(sourcePcdFilePath)
	if err != nil {
		return err
	}
	pc, err := pointcloud.ReadPCD(pcdFile)
	if err != nil {
		return err
	}

	im, _, err := ppRM.PointCloudToRGBD(pc)
	if err != nil {
		return err
	}

	var image bytes.Buffer
	if err := jpeg.Encode(&image, im, nil); err != nil {
		return err
	}

	err = os.WriteFile(args[2], image.Bytes(), 0o640)
	if err != nil {
		return err
	}

	return nil
}

func newPPRM(sourcePositionFilePath string) (transform.ParallelProjectionOntoXYWithRobotMarker, error) {
	data, err := os.ReadFile(sourcePositionFilePath)
	if err != nil {
		return transform.ParallelProjectionOntoXYWithRobotMarker{}, err
	}

	position, err := positionFromJSON(data)
	if err != nil {
		return transform.ParallelProjectionOntoXYWithRobotMarker{}, err
	}

	p := r3.Vector{X: position.Pose.X, Y: position.Pose.Y, Z: position.Pose.Z}

	quat := position.Extra.Quat
	orientation := &spatialmath.Quaternion{Real: quat.Real, Imag: quat.Imag, Jmag: quat.Jmag, Kmag: quat.Kmag}
	pose := spatialmath.NewPose(p, orientation)

	ppRM := transform.NewParallelProjectionOntoXYWithRobotMarker(&pose)
	return ppRM, nil

}

func positionFromJSON(data []byte) (position, error) {
	position := position{}

	if err := json.Unmarshal(data, &position); err != nil {
		return position, err
	}
	return position, nil
}
