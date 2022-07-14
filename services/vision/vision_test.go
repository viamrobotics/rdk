package vision

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func TestCloseService(t *testing.T) {
	ctx := context.Background()
	srv := createService(ctx, t, "data/empty.json")
	// success
	cfg := DetectorConfig{
		Name: "test",
		Type: "color",
		Parameters: config.AttributeMap{
			"detect_color": "#112233",
			"tolerance":    0.4,
			"segment_size": 100,
		},
	}
	err := srv.AddDetector(ctx, cfg)
	test.That(t, err, test.ShouldBeNil)
	vService := srv.(*visionService)
	fakeStruct := newStruct()
	det := func(image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{}, nil
	}
	registeredFn := registeredDetector{detector: det, closer: fakeStruct}
	logger := golog.NewTestLogger(t)
	err = vService.detReg.registerDetector("fake", &registeredFn, logger)
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

func createService(ctx context.Context, t *testing.T, filePath string) Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	srv, err := New(ctx, nil, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return srv
}
