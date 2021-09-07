package functionvm

import "github.com/go-errors/errors"

type engineCtor func() (Engine, error)

var engineRegistry = map[EngineName]engineCtor{}

// RegisterEngine registers the given engine name to an associated constructor.
func RegisterEngine(name EngineName, constructor engineCtor) {
	_, old := engineRegistry[name]
	if old {
		panic(errors.Errorf("trying to register two engines with same name %q", name))
	}
	engineRegistry[name] = constructor
}

// NewEngine makes a new engine of the given name.
func NewEngine(name EngineName) (Engine, error) {
	ctor, exists := engineRegistry[name]
	if !exists {
		return nil, errors.Errorf("no engine for %q", name)
	}
	return ctor()
}
