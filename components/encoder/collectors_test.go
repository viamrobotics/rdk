package encoder_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/encoder"
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
		ComponentName: "encoder",
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Target:        buf,
		Clock:         clock.New(),
	}

	enc := newEncoder()
	col, err := encoder.NewTicksCountCollector(enc, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
		Metadata: &datasyncpb.SensorMetadata{},
		Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
			"value":         1.0,
			"position_type": int(pb.PositionType_POSITION_TYPE_TICKS_COUNT),
		})},
	}})
	buf.Close()
}

func newEncoder() encoder.Encoder {
	e := &inject.Encoder{}
	e.PositionFunc = func(ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 1.0, encoder.PositionTypeTicks, nil
	}
	return e
}
