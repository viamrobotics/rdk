package servo_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	pb "go.viam.com/api/component/servo/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const captureInterval = time.Second

func TestServoCollector(t *testing.T) {
	mockClock := clk.NewMock()
	buf := tu.MockBuffer{}
	params := data.CollectorParams{
		ComponentName: "servo",
		Interval:      captureInterval,
		Logger:        logging.NewTestLogger(t),
		Target:        &buf,
		Clock:         mockClock,
	}

	serv := newServo()
	col, err := servo.NewPositionCollector(serv, params)
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

func newServo() servo.Servo {
	s := &inject.Servo{}
	s.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 1.0, nil
	}
	return s
}
