package datacapture

import (
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"io"
)

var (
	ErrStreamFull = errors.New("stream is full")
)

const (
	defaultChannelSize = 1000
)

type Stream struct {
	md *v1.DataCaptureMetadata
	c chan *v1.SensorData
	//spool *os.File
}

func NewStream(captureDir string, md *v1.DataCaptureMetadata) (*Stream, error) {
	//spool, err := CreateDataCaptureFile(captureDir, md)
	//if err != nil {
	//	return nil, err
	//}

	return &Stream{
		md: md,
		c:  make(chan *v1.SensorData, defaultChannelSize),
		//spool: spool,
	}, nil
}


func (s *Stream) Write(data *v1.SensorData) error {
	select {
	case s.c <- data :
			return nil
	default:
		return ErrStreamFull
	}
}

func (s *Stream) Read() (data *v1.SensorData, err error) {
	select {
	case x, ok := <-s.c:
		if ok {
			return x, nil
		} else {
			return nil, io.EOF
		}
	default:
		return nil, io.EOF
	}
}