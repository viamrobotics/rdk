package encoder

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	tu "go.viam.com/rdk/testutils"
)

const (
	componentName   = "encoder"
	captureInterval = time.Second
)

func TestEncoderCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: componentName,
		Interval:      captureInterval,
		Logger:        golog.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	enc := newEncoder(componentName)
	col, err := newTicksCountCollector(enc, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(captureInterval)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(buf.Writes), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble,
		tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
			Value:        1.0,
			PositionType: pb.PositionType_POSITION_TYPE_TICKS_COUNT,
		}))
}

type fakeEncoder struct {
	Encoder
	name resource.Name
}

func newEncoder(name string) Encoder {
	return &fakeEncoder{name: resource.Name{Name: name}}
}

func (e *fakeEncoder) Position(
	ctx context.Context,
	positionType PositionType,
	extra map[string]interface{},
) (float64, PositionType, error) {
	return 1.0, PositionTypeTicks, nil
}
