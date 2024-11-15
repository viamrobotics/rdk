package servo_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	captureInterval = time.Millisecond
)

func TestCollectors(t *testing.T) {
	start := time.Now()
	buf := tu.NewMockBuffer(t)
	params := data.CollectorParams{
		DataType:      data.CaptureTypeTabular,
		ComponentName: "servo",
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Target:        buf,
		Clock:         clock.New(),
	}

	serv := newServo()
	col, err := servo.NewPositionCollector(serv, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
		Metadata: &datasyncpb.SensorMetadata{},
		Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
			"position_deg": 1.0,
		})},
	}})
	buf.Close()
}

func newServo() servo.Servo {
	s := &inject.Servo{}
	s.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 1.0, nil
	}
	return s
}
