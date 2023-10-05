package servo

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	tu "go.viam.com/rdk/testutils"
)

const captureInterval = time.Second

func TestServoCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: "servo",
		Interval:      captureInterval,
		Logger:        golog.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	servo := newServo()
	col, err := newPositionCollector(servo, params)
	test.That(t, err, test.ShouldBeNil)

	defer col.Close()
	col.Collect()
	mockClock.Add(captureInterval)

	test.That(t, buf.Length(), test.ShouldEqual, 1)
	test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble,
		tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
			PositionDeg: 1.0,
		}))
}

type fakeServo struct {
	Servo
}

func newServo() Servo {
	return &fakeServo{}
}

func (s *fakeServo) Position(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	return 1.0, nil
}
