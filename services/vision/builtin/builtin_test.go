package builtin

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

func TestModelParameterSchema(t *testing.T) {
	ctx := context.Background()
	srv := makeService(ctx, t)
	// get parameters that exist
	params, err := srv.GetModelParameterSchema(ctx, RCSegmenter, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	parameterNames := params.Definitions["RadiusClusteringConfig"].Required
	test.That(t, parameterNames, test.ShouldContain, "min_points_in_plane")
	test.That(t, parameterNames, test.ShouldContain, "min_points_in_segment")
	test.That(t, parameterNames, test.ShouldContain, "clustering_radius_mm")
	test.That(t, parameterNames, test.ShouldContain, "mean_k_filtering")
	// attempt to get parameters that dont exist
	_, err = srv.GetModelParameterSchema(ctx, vision.VisModelType("not_a_model"), map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "do not have a schema for model type")
}

func TestCloseService(t *testing.T) {
	ctx := context.Background()
	srv := makeService(ctx, t)
	// success
	cfg := vision.VisModelConfig{
		Name: "test",
		Type: "color_detector",
		Parameters: config.AttributeMap{
			"detect_color":      "#112233",
			"hue_tolerance_pct": 0.4,
			"value_cutoff_pct":  0.2,
			"segment_size_px":   100,
		},
	}
	err := srv.AddDetector(ctx, cfg, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	vService := srv.(*builtIn)
	fakeStruct := newStruct()
	det := func(context.Context, image.Image) ([]objdet.Detection, error) {
		return []objdet.Detection{}, nil
	}
	registeredFn := registeredModel{Model: det, Closer: fakeStruct}
	logger := golog.NewTestLogger(t)
	err = vService.modReg.RegisterVisModel("fake", &registeredFn, logger)
	test.That(t, err, test.ShouldBeNil)
	err = viamutils.TryClose(ctx, srv)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fakeStruct.val, test.ShouldEqual, 1)

	detectors, err := srv.DetectorNames(ctx, map[string]interface{}{})
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

func makeService(ctx context.Context, t *testing.T) vision.Service {
	t.Helper()
	logger := golog.NewTestLogger(t)
	srv, err := NewBuiltIn(ctx, nil, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return srv
}
