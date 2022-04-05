package data

import (
	"os"

	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	v1 "go.viam.com/rdk/proto/api/service/datamanager/v1"
)

func ReadNextSensorData(f *os.File) (*v1.SensorData, error) {
	r := &v1.SensorData{}
	if _, err := pbutil.ReadDelimited(f, r); err != nil {
		return nil, err
	}
	return r, nil
}
