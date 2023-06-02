package fake

import (
	"bytes"
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	motion "go.viam.com/rdk/services/motion/builtin"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
)

func TestFakeBase(t *testing.T) {
	conf := resource.Config{
		Name: "test",
		Frame: &referenceframe.LinkConfig{
			Parent: referenceframe.World,
			Geometry: &spatialmath.GeometryConfig{
				R: 10,
			},
		},
	}

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	b, err := NewBase(ctx, conf)
	test.That(t, err, test.ShouldBeNil)
	fakeSLAM := fake.NewSLAM(slam.Named("test"), logger)

	// construct limits
	data, err := slam.GetPointCloudMapFull(ctx, fakeSLAM)
	test.That(t, err, test.ShouldBeNil)
	dims, err := pointcloud.GetPCDMetaData(bytes.NewReader(data))
	test.That(t, err, test.ShouldBeNil)
	limits := []referenceframe.Limit{
		{Min: dims.MinX, Max: dims.MaxX},
		{Min: dims.MinY, Max: dims.MaxY},
		{Min: -2 * math.Pi, Max: 2 * math.Pi},
	}

	// construct localizer
	localizer := &motion.SLAMLocalizer{
		Service: fakeSLAM,
		Limits:  limits,
	}

	kb, err := b.(*Base).WrapWithKinematics(ctx, localizer)
	test.That(t, err, test.ShouldBeNil)
	expected := referenceframe.FloatsToInputs([]float64{10, 11, 0})
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	inputs, err := kb.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, expected)
}
