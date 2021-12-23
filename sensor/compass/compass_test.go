package compass_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"

	"go.viam.com/rdk/sensor/compass"
	"go.viam.com/rdk/testutils/inject"

	"go.viam.com/test"
)

func TestMedianHeading(t *testing.T) {
	dev := &inject.Compass{}
	err1 := errors.New("whoops")
	dev.HeadingFunc = func(ctx context.Context) (float64, error) {
		return 0, err1
	}
	_, err := compass.MedianHeading(context.Background(), dev)
	test.That(t, err, test.ShouldEqual, err1)

	readings := []float64{1, 2, 3, 4, 4, 2, 4, 4, 1, 1, 2}
	readCount := 0
	dev.HeadingFunc = func(ctx context.Context) (float64, error) {
		reading := readings[readCount]
		readCount++
		return reading, nil
	}
	med, err := compass.MedianHeading(context.Background(), dev)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, med, test.ShouldEqual, 3)
}
