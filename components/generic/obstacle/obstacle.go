// Package obstacle implements a builtin generic component model that places a static obstacle,
// described by a list of geometries, in the frame system so that motion planning avoids it.
package obstacle

import (
	"context"
	"fmt"
	"strings"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// Model is the model of a static obstacle. Its "geometries" attribute lists geometry configs in the
// same shape as a frame's "geometry"; each one's translation and orientation position it relative
// to the component's frame, and its label names it. The geometries reach the frame system as
// "<component>:<label>", so other resources can be parented to them and collision specifications
// can name them individually or the component as a whole.
var Model = resource.DefaultModelFamily.WithModel("obstacle")

// Config describes the obstacle.
type Config struct {
	Geometries []spatialmath.GeometryConfig `json:"geometries"`
}

// Validate ensures all parts of the config are valid.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if len(c.Geometries) == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "geometries")
	}
	labels := make(map[string]struct{}, len(c.Geometries))
	for i, gc := range c.Geometries {
		geometryPath := fmt.Sprintf("%s.geometries.%d", path, i)
		if _, err := gc.ParseConfig(); err != nil {
			return nil, nil, resource.NewConfigValidationError(geometryPath, err)
		}
		if gc.Label == "" {
			continue
		}
		if gc.Label == referenceframe.World {
			return nil, nil, resource.NewConfigValidationError(geometryPath, fmt.Errorf("%q is a reserved label", gc.Label))
		}
		if strings.Contains(gc.Label, ":") {
			return nil, nil, resource.NewConfigValidationError(geometryPath, fmt.Errorf("label %q may not contain ':'", gc.Label))
		}
		if _, dup := labels[gc.Label]; dup {
			return nil, nil, resource.NewConfigValidationError(geometryPath, fmt.Errorf("label %q is used more than once", gc.Label))
		}
		labels[gc.Label] = struct{}{}
	}
	return nil, nil, nil
}

// parseGeometries turns the configs into geometries. Unlabeled geometries are named by position so
// every one has a stable frame system name.
func (c *Config) parseGeometries() ([]spatialmath.Geometry, error) {
	geometries := make([]spatialmath.Geometry, 0, len(c.Geometries))
	for i, gc := range c.Geometries {
		g, err := gc.ParseConfig()
		if err != nil {
			return nil, err
		}
		if g.Label() == "" {
			g.SetLabel(fmt.Sprintf("geometry_%d", i))
		}
		geometries = append(geometries, g)
	}
	return geometries, nil
}

func init() {
	resource.RegisterComponent(generic.API, Model, resource.Registration[resource.Resource, *Config]{Constructor: newObstacle})
}

// obstacle is immutable after construction; a config change rebuilds it.
type obstacle struct {
	resource.Named
	resource.TriviallyCloseable
	logger     logging.Logger
	geometries []spatialmath.Geometry
	model      referenceframe.Model
}

func newObstacle(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (resource.Resource, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	geometries, err := newConf.parseGeometries()
	if err != nil {
		return nil, err
	}
	model, err := referenceframe.NewModelFromGeometries(conf.ResourceName().ShortName(), geometries)
	if err != nil {
		return nil, err
	}

	switch {
	case conf.Frame == nil:
		logger.CWarn(ctx, "obstacle has no frame configured; it will not be part of the frame system "+
			"and motion planning will not avoid it")
	case conf.Frame.Geometry != nil:
		logger.CWarn(ctx, "obstacle has a geometry in its frame config; the frame system ignores it in favor "+
			"of the obstacle's geometries")
	}

	return &obstacle{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		geometries: geometries,
		model:      model,
	}, nil
}

// Kinematics returns the zero-DoF model that carries the obstacle's geometries into the frame system.
func (o *obstacle) Kinematics(context.Context) (referenceframe.Model, error) {
	return o.model, nil
}

// Geometries returns copies of the obstacle's geometries, positioned in the component's frame.
func (o *obstacle) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	out := make([]spatialmath.Geometry, 0, len(o.geometries))
	for _, g := range o.geometries {
		out = append(out, g.Transform(spatialmath.NewZeroPose()))
	}
	return out, nil
}
