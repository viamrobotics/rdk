// Package main is a module that utilizes both required and optional implicit
// dependencies. It serves a generic component that has a required dependency on one motor
// and an optional dependency on another motor. It also serves a generic component that
// exhibits optional dependency cycles.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/module/trace"
	"go.viam.com/rdk/resource"
)

var (
	fooModel           = resource.NewModel("acme", "demo", "foo")
	mocModel           = resource.NewModel("acme", "demo", "moc" /* "mutual optional child" */)
	pointerTargetModel = resource.NewModel("acme", "demo", "pointer-target")
	pointerHolderModel = resource.NewModel("acme", "demo", "pointer-holder")
)

func main() {
	resource.RegisterComponent(generic.API, fooModel, resource.Registration[resource.Resource, *FooConfig]{
		Constructor: newFoo,
	})
	resource.RegisterComponent(generic.API, mocModel, resource.Registration[resource.Resource, *MutualOptionalChildConfig]{
		Constructor: newMutualOptionalChild,
	})
	resource.RegisterComponent(generic.API, pointerTargetModel, resource.Registration[resource.Resource, *PointerTargetConfig]{
		Constructor: newPointerTarget,
	})
	resource.RegisterComponent(generic.API, pointerHolderModel, resource.Registration[resource.Resource, *PointerHolderConfig]{
		Constructor: newPointerHolder,
	})

	module.ModularMain(resource.APIModel{generic.API, fooModel},
		resource.APIModel{generic.API, mocModel},
		resource.APIModel{generic.API, pointerTargetModel},
		resource.APIModel{generic.API, pointerHolderModel})
}

// FooConfig contains a required and optional motor that the component will necessarily
// and optionally depend upon respectively.
type FooConfig struct {
	RequiredMotor string `json:"required_motor"`
	OptionalMotor string `json:"optional_motor"`
}

// Validate validates the config and returns a required dependency on `required_motor` and
// an optional dependency on `optional_motor`.
func (fCfg *FooConfig) Validate(path string) ([]string, []string, error) {
	var requiredDeps, optionalDeps []string

	if fCfg.RequiredMotor == "" {
		return nil, nil,
			fmt.Errorf(`expected "required_motor" attribute for foo %q`, path)
	}
	requiredDeps = append(requiredDeps, fCfg.RequiredMotor)

	if fCfg.OptionalMotor != "" {
		optionalDeps = append(optionalDeps, fCfg.OptionalMotor)
	}

	return requiredDeps, optionalDeps, nil
}

type foo struct {
	resource.Named
	resource.TriviallyCloseable

	logger logging.Logger

	requiredMotor motor.Motor
	optionalMotor motor.Motor
}

func newFoo(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	ctx, span := trace.StartSpan(ctx, "optionaldepsmodule::newFoo")
	defer span.End()
	f := &foo{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := f.reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return f, nil
}

func (f *foo) reconfigure(ctx context.Context, deps resource.Dependencies,
	conf resource.Config,
) error {
	fooConfig, err := resource.NativeConfig[*FooConfig](conf)
	if err != nil {
		return err
	}

	f.requiredMotor, err = motor.FromProvider(deps, fooConfig.RequiredMotor)
	if err != nil {
		return fmt.Errorf("could not get required motor %s from dependencies",
			fooConfig.RequiredMotor)
	}

	// Resolve the optional motor by name. If the config value is a fully qualified
	// resource name (e.g. "rdk:component:motor/m"), parse it directly to get the
	// correct lookup key; otherwise fall back to motor.Named for short/remote names.
	optMotorName := motor.Named(fooConfig.OptionalMotor)
	if parsed, parseErr := resource.NewFromString(fooConfig.OptionalMotor); parseErr == nil {
		optMotorName = parsed
	}
	f.optionalMotor, err = resource.FromProvider[motor.Motor](deps, optMotorName)
	if err != nil {
		f.logger.Infof("could not get optional motor %s from dependencies; continuing",
			fooConfig.OptionalMotor)
	}

	return nil
}

// DoCommand is the only method of this component; shows how one might leverage the
// required and optional motor dependencies.
func (f *foo) DoCommand(ctx context.Context, req map[string]any) (map[string]any, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	// "required_motor_state" will check the state of the required motor.
	if cmd == "required_motor_state" {
		if f.requiredMotor == nil {
			return map[string]any{"required_motor_state": "unset"}, nil
		}

		requiredMotorIsMoving, err := f.requiredMotor.IsMoving(ctx)
		if err != nil {
			return map[string]any{"required_motor_state": "unreachable"}, nil //nolint:nilerr
		}
		return map[string]any{"required_motor_state": fmt.Sprintf("moving: %v", requiredMotorIsMoving)}, nil
	}

	// "optional_motor_state" will check the state of the optional motor.
	if cmd == "optional_motor_state" {
		if f.optionalMotor == nil {
			return map[string]any{"optional_motor_state": "unset"}, nil
		}

		optionalMotorIsMoving, err := f.optionalMotor.IsMoving(ctx)
		if err != nil {
			return map[string]any{"optional_motor_state": "unreachable"}, nil //nolint:nilerr
		}
		return map[string]any{"optional_motor_state": fmt.Sprintf("moving: %v", optionalMotorIsMoving)}, nil
	}

	// The command must've been something else (unrecognized).
	return nil, fmt.Errorf("unknown command string %s", cmd)
}

// MutualOptionalChildConfig contains _another_ MOC that this MOC will optionally depend
// upon.
type MutualOptionalChildConfig struct {
	OtherMOC string `json:"other_moc"`
}

// Validate validates the config and returns an optional dependency on `other_moc`.
//
//nolint:unparam
func (mocCfg *MutualOptionalChildConfig) Validate(path string) ([]string, []string, error) {
	if mocCfg.OtherMOC == "" {
		return nil, nil,
			fmt.Errorf(`expected "other_moc" attribute for MOC %q`, path)
	}
	return nil, []string{mocCfg.OtherMOC}, nil
}

// mocInstanceCounter gives each mutualOptionalChild instance a unique ID so tests can
// detect whether a captured `otherMOC` pointer still references the current instance.
var mocInstanceCounter atomic.Uint64

type mutualOptionalChild struct {
	resource.Named
	resource.TriviallyCloseable
	resource.AlwaysRebuild

	logger logging.Logger

	instanceID uint64
	otherMOC   resource.Resource
}

func newMutualOptionalChild(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	moc := &mutualOptionalChild{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		instanceID: mocInstanceCounter.Add(1),
	}

	mutualOptionalChildConfig, err := resource.NativeConfig[*MutualOptionalChildConfig](conf)
	if err != nil {
		return nil, err
	}

	moc.otherMOC, err = generic.FromProvider(deps, mutualOptionalChildConfig.OtherMOC)
	if err != nil {
		moc.logger.Infof("could not get other MOC %s from dependencies; continuing",
			mutualOptionalChildConfig.OtherMOC)
	}

	return moc, nil
}

// DoCommand is the only method of this component.
func (moc *mutualOptionalChild) DoCommand(ctx context.Context, req map[string]any) (map[string]any, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	// "other_moc_state" will check the state of the required motor.
	if cmd == "other_moc_state" {
		if moc.otherMOC == nil {
			return map[string]any{"other_moc_state": "unset"}, nil
		}

		resp, err := moc.otherMOC.DoCommand(ctx, map[string]any{"command": "query"})
		if err != nil {
			return map[string]any{"other_moc_state": "unreachable"}, nil //nolint:nilerr
		}

		if _, exists := resp["usable"]; exists {
			return map[string]any{
				"other_moc_state":   "usable",
				"other_instance_id": resp["instance_id"],
			}, nil
		}
		return map[string]any{"other_moc_state": "unusable"}, nil
	}

	// "query" responds with {"usable": nil, "instance_id": <id>}; tests use the ID to
	// distinguish a fresh captured pointer from a stale one.
	if cmd == "query" {
		return map[string]any{"usable": nil, "instance_id": moc.instanceID}, nil
	}

	// "instance_id" returns just the ID so tests can fetch the current instance's ID
	// without going through a dependent's captured pointer.
	if cmd == "instance_id" {
		return map[string]any{"instance_id": moc.instanceID}, nil
	}

	// The command must've been something else (unrecognized).
	return nil, fmt.Errorf("unknown command string %s", cmd)
}

// `moc` is notably missing a `Reconfigure` method. Modular resources with optional
// dependencies should be able to leverage optional dependencies even as "always rebuild"
// resources.

// PointerTargetConfig configures a pointer-target: a module-served resource that carries
// an optional dependency, making it eligible for rebuild by the RDK's
// updateWeakAndOptionalDependents flow. Another module-internal resource (a pointer-holder)
// can capture a direct Go pointer to it and exercise stale-pointer scenarios when the
// target is rebuilt via the weak/optional path.
type PointerTargetConfig struct {
	OptionalDep string `json:"optional_dep"`
}

// Validate returns the optional dependency so updateWeakAndOptionalDependents considers this
// resource for rebuilding.
func (c *PointerTargetConfig) Validate(path string) ([]string, []string, error) {
	var optionalDeps []string
	if c.OptionalDep != "" {
		optionalDeps = append(optionalDeps, c.OptionalDep)
	}
	return nil, optionalDeps, nil
}

// pointerTargetInstanceCounter gives each pointerTarget instance a unique ID so tests can
// verify whether a pointer-holder's captured Go pointer references the latest instance or
// a stale one.
var pointerTargetInstanceCounter atomic.Uint64

// pointerTarget tracks whether Close was called and carries a unique instance ID.
// DoCommand returns an error if called after Close — this is what lets a dependent detect
// a stale pointer to a closed instance. When alive, it returns its instance ID so callers
// can verify they are talking to the latest instance.
type pointerTarget struct {
	resource.Named
	resource.AlwaysRebuild
	instanceID uint64
	closed     atomic.Bool
}

func newPointerTarget(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger) (resource.Resource, error) {
	return &pointerTarget{
		Named:      conf.ResourceName().AsNamed(),
		instanceID: pointerTargetInstanceCounter.Add(1),
	}, nil
}

func (p *pointerTarget) Close(_ context.Context) error {
	p.closed.Store(true)
	return nil
}

func (p *pointerTarget) DoCommand(_ context.Context, _ map[string]any) (map[string]any, error) {
	if p.closed.Load() {
		return nil, errors.New("pointerTarget is closed")
	}
	return map[string]any{"instance_id": p.instanceID}, nil
}

// PointerHolderConfig configures a pointer-holder: a resource that explicitly depends on
// a pointer-target and captures a direct Go pointer to it at construction time.
type PointerHolderConfig struct {
	Target string `json:"target"`
}

// Validate declares target as a required dependency so it gets resolved via `deps` at
// construction time and a Go pointer can be captured.
func (c *PointerHolderConfig) Validate(path string) ([]string, []string, error) {
	if c.Target == "" {
		return nil, nil, fmt.Errorf(`expected "target" attribute for pointer-holder %q`, path)
	}
	return []string{c.Target}, nil, nil
}

// pointerHolder holds a direct Go pointer to a pointerTarget captured at construction time.
// Its DoCommand proxies through the stored pointer — this is the pattern that leaves the
// holder with a stale reference if the target is rebuilt without notifying the holder.
type pointerHolder struct {
	resource.Named
	resource.TriviallyCloseable
	resource.AlwaysRebuild
	target resource.Resource
}

func newPointerHolder(_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger) (resource.Resource, error) {
	cfg, err := resource.NativeConfig[*PointerHolderConfig](conf)
	if err != nil {
		return nil, err
	}
	target, ok := deps[generic.Named(cfg.Target)]
	if !ok {
		return nil, fmt.Errorf("pointer-holder could not find target %q in dependencies", cfg.Target)
	}
	return &pointerHolder{Named: conf.ResourceName().AsNamed(), target: target}, nil
}

func (p *pointerHolder) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	return p.target.DoCommand(ctx, cmd)
}
