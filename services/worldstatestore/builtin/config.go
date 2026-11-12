package builtin

import (
	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/resource"
)

const transformsKey = "transforms"

// Config is the configuration of the in-memory world state store.
type Config struct {
	// Transforms are the transforms the store holds when it starts, for the parts of the world
	// that are known ahead of time and should survive a restart. Each is a proto JSON transform,
	// exactly as `DoAdd` takes it:
	//
	//	{"transforms": [{"referenceFrame": "table", "poseInObserverFrame": {...}, "physicalObject": {...}}]}
	//
	// A transform without a uuid uses its reference frame name as one. That keeps its identity
	// the same each time the store is built, which a random uuid would not.
	//
	// Changing this list rebuilds the store, which discards everything added since it started.
	Transforms []map[string]any `json:"transforms,omitempty"`
}

// Validate checks that the configured transforms could all be added to an empty store.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	scratch := &builtIn{transforms: make(map[string]*commonpb.Transform)}
	if err := scratch.seed(conf.Transforms); err != nil {
		return nil, nil, resource.NewConfigValidationError(path, err)
	}
	return nil, nil, nil
}

// seed adds the configured transforms to the store.
func (svc *builtIn) seed(transforms []map[string]any) error {
	if len(transforms) == 0 {
		return nil
	}

	events, err := decodeEvents(transformsKey, transforms)
	if err != nil {
		return err
	}
	for _, event := range events {
		transform := event.GetTransform()
		if len(transform.GetUuid()) == 0 {
			transform.Uuid = []byte(transform.GetReferenceFrame())
		}
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	_, err = svc.addLocked(transformsKey, events)
	return err
}
