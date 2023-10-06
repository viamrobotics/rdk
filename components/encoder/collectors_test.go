package encoder_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/data"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const captureInterval = time.Second

func TestEncoderCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: "encoder",
		Interval:      captureInterval,
		Logger:        golog.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	enc := newEncoder()
	col, err := encoder.NewTicksCountCollector(enc, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(captureInterval)

	test.That(t, buf.Length(), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble,
		tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
			Value:        1.0,
			PositionType: pb.PositionType_POSITION_TYPE_TICKS_COUNT,
		}))
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
