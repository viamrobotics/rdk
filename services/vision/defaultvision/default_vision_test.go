package defaultvision

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/vision"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestCloseService(t *testing.T) {
	ctx := context.Background()
	srv := createService(ctx, t)
	// success
	cfg := vision.VisModelConfig{
		Name: "test",
		Type: "color_detector",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	err := srv.AddDetector(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	vService := srv.(*visionDefaultService)
	fakeStruct := newStruct()
	det := func(context.Context, image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{}, nil
	}
	// registeredFn := registeredDetector{detector: det, closer: fakeStruct}
	registeredFn := vision.RegisteredModel{Model: det, Closer: fakeStruct}
	logger := golog.NewTestLogger(t)
	err = vService.modReg.RegisterVisModel("fake", &registeredFn, logger)
	test.That(t, err, test.ShouldBeNil)
	err = viamutils.TryClose(ctx, srv)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeStruct.val, test.ShouldEqual, 1)

	detectors, err := srv.GetDetectorNames(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(detectors), test.ShouldEqual, 0)
}

func newStruct() *fakeClosingStruct {
	return &fakeClosingStruct{val: 0}
}

type fakeClosingStruct struct {
	val int
}

func (s *fakeClosingStruct) Close() error {
	s.val++
	return nil
}

func createService(ctx context.Context, t *testing.T) vision.Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	srv, err := NewDefault(ctx, nil, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return srv
}
