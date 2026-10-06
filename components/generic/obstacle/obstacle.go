// Package obstacle implements a builtin generic component model that places a static obstacle,
// described by a list of geometries, in the frame system so that motion planning avoids it.
package obstacle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

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

// DoCommand keys and values understood by the obstacle component.
const (
	CommandKey    = "command"
	CommandGet    = "get"
	CommandSet    = "set"
	GeometriesKey = "geometries"
)

// Config describes the obstacle.
type Config struct {
	Geometries []spatialmath.GeometryConfig `json:"geometries"`
}

// Validate ensures all parts of the config are valid.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if len(c.Geometries) == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, GeometriesKey)
	}
	labels := make(map[string]struct{}, len(c.Geometries))
	for i, gc := range c.Geometries {
		geometryPath := fmt.Sprintf("%s.%s.%d", path, GeometriesKey, i)
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

type obstacle struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger

	mu         sync.RWMutex
	cfg        Config
	geometries []spatialmath.Geometry
	model      referenceframe.Model
}

func newObstacle(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (resource.Resource, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	o := &obstacle{Named: conf.ResourceName().AsNamed(), logger: logger}
	if err := o.apply(*newConf); err != nil {
		return nil, err
	}

	switch {
	case conf.Frame == nil:
		logger.CWarn(ctx, "obstacle has no frame configured; it will not be part of the frame system "+
			"and motion planning will not avoid it")
	case conf.Frame.Geometry != nil:
		logger.CWarn(ctx, "obstacle has a geometry in its frame config; the frame system will use that "+
			"geometry instead of the obstacle's geometries")
	}
	return o, nil
}

// apply replaces the obstacle's contents with those described by cfg. The caller holds the lock
// when the obstacle is already in use.
func (o *obstacle) apply(cfg Config) error {
	geometries, err := cfg.parseGeometries()
	if err != nil {
		return err
	}
	model, err := referenceframe.NewModelFromGeometries(o.Name().ShortName(), geometries)
	if err != nil {
		return err
	}
	o.cfg = cfg
	o.geometries = geometries
	o.model = model
	return nil
}

// Kinematics returns the zero-DoF model that carries the obstacle's geometries into the frame system.
func (o *obstacle) Kinematics(context.Context) (referenceframe.Model, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.model, nil
}

// Geometries returns copies of the obstacle's geometries, positioned in the component's frame.
func (o *obstacle) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]spatialmath.Geometry, 0, len(o.geometries))
	for _, g := range o.geometries {
		out = append(out, g.Transform(spatialmath.NewZeroPose()))
	}
	return out, nil
}

// DoCommand reads or replaces the obstacle's geometries at runtime.
//
// {"command": "get"} returns the current geometries.
//
// {"command": "set", "geometries": [...]} replaces the geometries with the given list, in the same
// shape as the config attribute, after validating it. Changes are visible through Geometries,
// Kinematics and the robot's FrameSystemConfig RPC right away, but the builtin motion service
// works from frame system parts cached at the last reconfigure, so it keeps planning against the
// previous geometries until the machine reconfigures. A reconfigure also resets the obstacle to its
// configured geometries.
func (o *obstacle) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, _ := cmd[CommandKey].(string)
	switch command {
	case CommandGet:
		o.mu.RLock()
		defer o.mu.RUnlock()
		return o.statusLocked()
	case CommandSet:
		raw, present := cmd[GeometriesKey]
		if !present {
			return nil, fmt.Errorf("%q command requires %q", CommandSet, GeometriesKey)
		}
		geometries, err := geometriesFromCommand(raw)
		if err != nil {
			return nil, err
		}
		newCfg := Config{Geometries: geometries}
		if _, _, err := newCfg.Validate(""); err != nil {
			return nil, err
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		if err := o.apply(newCfg); err != nil {
			return nil, err
		}
		return o.statusLocked()
	default:
		return nil, fmt.Errorf("unknown %q %q; supported commands are %q and %q", CommandKey, command, CommandGet, CommandSet)
	}
}

func (o *obstacle) statusLocked() (map[string]interface{}, error) {
	geometries, err := toJSONValue(o.cfg.Geometries)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{GeometriesKey: geometries}, nil
}

// geometriesFromCommand decodes the geometries list of a DoCommand through JSON, so it accepts
// both a decoded JSON array and Go values with the same shape.
func geometriesFromCommand(raw interface{}) ([]spatialmath.GeometryConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", GeometriesKey, err)
	}
	var geometries []spatialmath.GeometryConfig
	if err := json.Unmarshal(data, &geometries); err != nil {
		return nil, fmt.Errorf("%s must be a list of geometry configs: %w", GeometriesKey, err)
	}
	if geometries == nil {
		return nil, errors.New(GeometriesKey + " must be a list of geometry configs")
	}
	return geometries, nil
}

func toJSONValue(v interface{}) (interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
