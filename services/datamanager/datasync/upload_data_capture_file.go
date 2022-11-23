package datasync

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadDataCaptureFile(ctx context.Context, client v1.DataSyncServiceClient, f *datacapture.File, partID string) error {
	// Send metadata syncQueue request.
	fmt.Println("starting data capture upload")
	defer func() {
		fmt.Println("finished data capture")
	}()
	md := f.ReadMetadata()
	f.Reset()
	sensorData, err := getAllSensorData(f)
	fmt.Println("got all sensor data: ")
	fmt.Println(sensorData)
	if err != nil {
		fmt.Println("failed to get all sensor data")
		fmt.Println(err)
		return err
	}

	ur := &v1.DataCaptureUploadRequest{
		Metadata: &v1.UploadMetadata{
			PartId:           partID,
			ComponentType:    md.GetComponentType(),
			ComponentName:    md.GetComponentName(),
			ComponentModel:   md.GetComponentModel(),
			MethodName:       md.GetMethodName(),
			Type:             md.GetType(),
			MethodParameters: md.GetMethodParameters(),
			Tags:             md.GetTags(),
			SessionId:        md.GetSessionId(),
		},
		SensorContents: sensorData,
	}
	resp, err := client.DataCaptureUpload(ctx, ur)
	if err != nil {
		fmt.Println("got error during data capture upload")
		return err
	}
	// TODO: real code handling
	if resp.GetCode() != 200 {
		fmt.Println("got non-200 during data capture upload")
		return errors.New("error making syncQueue request: " + resp.GetMessage())
	}
	return nil
}

func getAllSensorData(f *datacapture.File) ([]*v1.SensorData, error) {
	fmt.Println("getting sensor data from file with size")
	fmt.Println(f.Size())
	var ret []*v1.SensorData
	for {
		next, err := f.ReadNext()
		if err != nil {
			fmt.Println(err)
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		ret = append(ret, next)
	}
	return ret, nil
}
