// Package obstacle implements builtin generic component models that place static obstacles in the
// frame system so that motion planning avoids them.
package obstacle

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// BoundsModel is the model of a box-shaped enclosure of obstacle walls. It is positioned with the
// component's frame config, which sits at the center of the interior, and contributes up to six
// faces to the frame system as geometries named "<frame>:x_max", "<frame>:x_min", "<frame>:y_max",
// "<frame>:y_min", "<frame>:floor" and "<frame>:ceiling".
var BoundsModel = resource.DefaultModelFamily.WithModel("bounds")

const defaultWallThicknessMM = 10.0

// Face labels, in the order WallGeometries emits them.
const (
	LabelXMax    = "x_max"
	LabelXMin    = "x_min"
	LabelYMax    = "y_max"
	LabelYMin    = "y_min"
	LabelFloor   = "floor"
	LabelCeiling = "ceiling"
)

// FaceLabels lists every face label accepted by the "exclude" attribute.
var FaceLabels = []string{LabelXMax, LabelXMin, LabelYMax, LabelYMin, LabelFloor, LabelCeiling}

// DoCommand keys and values understood by the bounds component.
const (
	CommandKey = "command"
	CommandGet = "get"
	CommandSet = "set"
)

// Config describes the enclosure. Dimensions are the interior clearance in millimeters; walls are
// added outside of it. Exclude names the faces to leave out, e.g. ["floor", "x_min"].
type Config struct {
	XMm             float64  `json:"x_mm"`
	YMm             float64  `json:"y_mm"`
	ZMm             float64  `json:"z_mm"`
	WallThicknessMm float64  `json:"wall_thickness_mm"`
	Exclude         []string `json:"exclude,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (c *Config) Validate(path string) ([]string, []string, error) {
	for _, dim := range []struct {
		field string
		value float64
	}{{"x_mm", c.XMm}, {"y_mm", c.YMm}, {"z_mm", c.ZMm}} {
		if dim.value == 0 {
			return nil, nil, resource.NewConfigValidationFieldRequiredError(path, dim.field)
		}
		if err := validatePositive(dim.field, dim.value); err != nil {
			return nil, nil, resource.NewConfigValidationError(path, err)
		}
	}
	if c.WallThicknessMm != 0 {
		if err := validatePositive("wall_thickness_mm", c.WallThicknessMm); err != nil {
			return nil, nil, resource.NewConfigValidationError(path, err)
		}
	}
	seen := make(map[string]struct{}, len(c.Exclude))
	for _, face := range c.Exclude {
		if !slices.Contains(FaceLabels, face) {
			return nil, nil, resource.NewConfigValidationError(path,
				fmt.Errorf("exclude contains unknown face %q; faces are %v", face, FaceLabels))
		}
		if _, dup := seen[face]; dup {
			return nil, nil, resource.NewConfigValidationError(path, fmt.Errorf("exclude lists face %q more than once", face))
		}
		seen[face] = struct{}{}
	}
	if len(seen) == len(FaceLabels) {
		return nil, nil, resource.NewConfigValidationError(path, errors.New("exclude leaves no faces"))
	}
	return nil, nil, nil
}

func validatePositive(field string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("%s must be a positive number, got %v", field, value)
	}
	return nil
}

// withDefaults returns a copy of the config with every optional field resolved.
func (c Config) withDefaults() Config {
	if c.WallThicknessMm == 0 {
		c.WallThicknessMm = defaultWallThicknessMM
	}
	return c
}

type wall struct {
	label  string
	center r3.Vector
	dims   r3.Vector
}

// WallGeometries returns the box geometries of the enclosure described by the config, in the
// component's frame, which sits at the center of the interior. Walls extend past each other at the
// edges so the shell is closed.
func WallGeometries(c Config) ([]spatialmath.Geometry, error) {
	c = c.withDefaults()
	x, y, z, t := c.XMm, c.YMm, c.ZMm, c.WallThicknessMm

	walls := []wall{
		{LabelXMax, r3.Vector{X: x/2 + t/2, Y: 0, Z: 0}, r3.Vector{X: t, Y: y + 2*t, Z: z + 2*t}},
		{LabelXMin, r3.Vector{X: -(x/2 + t/2), Y: 0, Z: 0}, r3.Vector{X: t, Y: y + 2*t, Z: z + 2*t}},
		{LabelYMax, r3.Vector{X: 0, Y: y/2 + t/2, Z: 0}, r3.Vector{X: x + 2*t, Y: t, Z: z + 2*t}},
		{LabelYMin, r3.Vector{X: 0, Y: -(y/2 + t/2), Z: 0}, r3.Vector{X: x + 2*t, Y: t, Z: z + 2*t}},
		{LabelFloor, r3.Vector{X: 0, Y: 0, Z: -(z/2 + t/2)}, r3.Vector{X: x + 2*t, Y: y + 2*t, Z: t}},
		{LabelCeiling, r3.Vector{X: 0, Y: 0, Z: z/2 + t/2}, r3.Vector{X: x + 2*t, Y: y + 2*t, Z: t}},
	}

	geometries := make([]spatialmath.Geometry, 0, len(walls))
	for _, w := range walls {
		if slices.Contains(c.Exclude, w.label) {
			continue
		}
		box, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(w.center), w.dims, w.label)
		if err != nil {
			return nil, err
		}
		geometries = append(geometries, box)
	}
	return geometries, nil
}

func init() {
	resource.RegisterComponent(generic.API, BoundsModel, resource.Registration[resource.Resource, *Config]{Constructor: newBounds})
}

type bounds struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger

	mu         sync.RWMutex
	cfg        Config
	geometries []spatialmath.Geometry
}

func newBounds(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger) (resource.Resource, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	cfg := newConf.withDefaults()
	geometries, err := WallGeometries(cfg)
	if err != nil {
		return nil, err
	}

	switch {
	case conf.Frame == nil:
		logger.CWarn(ctx, "bounds component has no frame configured; it will not be part of the frame system "+
			"and motion planning will not avoid it")
	case conf.Frame.Geometry != nil:
		logger.CWarn(ctx, "bounds component has a geometry in its frame config; the frame system will use that "+
			"geometry instead of the enclosure walls")
	}

	return &bounds{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cfg:        cfg,
		geometries: geometries,
	}, nil
}

// Geometries returns copies of the enclosure walls in the component's frame.
func (b *bounds) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return copyGeometries(b.geometries), nil
}

// DoCommand reads or adjusts the enclosure at runtime.
//
// {"command": "get"} returns the current configuration and the number of wall geometries.
//
// {"command": "set", ...} accepts any subset of the config attributes ("x_mm", "y_mm", "z_mm",
// "wall_thickness_mm", "exclude"), validates the merged result and rebuilds the walls. Changes are
// visible through Geometries and the robot's FrameSystemConfig RPC right away, but the builtin
// motion service works from frame system parts cached at the last reconfigure, so it keeps planning
// against the previous walls until the machine reconfigures. A reconfigure also resets the
// enclosure to its configured attributes.
func (b *bounds) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, _ := cmd[CommandKey].(string)
	switch command {
	case CommandGet:
		b.mu.RLock()
		defer b.mu.RUnlock()
		return b.statusLocked(), nil
	case CommandSet:
		b.mu.Lock()
		defer b.mu.Unlock()
		newCfg := b.cfg
		if err := applyOverrides(&newCfg, cmd); err != nil {
			return nil, err
		}
		if _, _, err := newCfg.Validate(""); err != nil {
			return nil, err
		}
		newCfg = newCfg.withDefaults()
		geometries, err := WallGeometries(newCfg)
		if err != nil {
			return nil, err
		}
		b.cfg = newCfg
		b.geometries = geometries
		return b.statusLocked(), nil
	default:
		return nil, fmt.Errorf("unknown %q %q; supported commands are %q and %q", CommandKey, command, CommandGet, CommandSet)
	}
}

func (b *bounds) statusLocked() map[string]interface{} {
	return map[string]interface{}{
		"x_mm":              b.cfg.XMm,
		"y_mm":              b.cfg.YMm,
		"z_mm":              b.cfg.ZMm,
		"wall_thickness_mm": b.cfg.WallThicknessMm,
		"exclude":           slices.Clone(b.cfg.Exclude),
		"geometry_count":    len(b.geometries),
	}
}

// applyOverrides copies the attributes present in cmd onto cfg. JSON numbers arrive as float64.
func applyOverrides(cfg *Config, cmd map[string]interface{}) error {
	for key, target := range map[string]*float64{
		"x_mm":              &cfg.XMm,
		"y_mm":              &cfg.YMm,
		"z_mm":              &cfg.ZMm,
		"wall_thickness_mm": &cfg.WallThicknessMm,
	} {
		raw, present := cmd[key]
		if !present {
			continue
		}
		value, ok := raw.(float64)
		if !ok {
			return fmt.Errorf("%s must be a number, got %T", key, raw)
		}
		*target = value
	}
	if raw, present := cmd["exclude"]; present {
		exclude, err := stringSlice(raw)
		if err != nil {
			return fmt.Errorf("exclude %w", err)
		}
		cfg.Exclude = exclude
	}
	return nil
}

// stringSlice accepts the two shapes a list of strings takes in a DoCommand: a decoded JSON array
// ([]interface{}) or a Go []string.
func stringSlice(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []string:
		return slices.Clone(v), nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("must be a list of strings, got element %T", item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("must be a list of strings, got %T", raw)
	}
}

func copyGeometries(geometries []spatialmath.Geometry) []spatialmath.Geometry {
	out := make([]spatialmath.Geometry, 0, len(geometries))
	for _, g := range geometries {
		out = append(out, g.Transform(spatialmath.NewZeroPose()))
	}
	return out
}
