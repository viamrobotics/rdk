package functionvm

import (
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

type engineCtor func() (Engine, error)

var engineRegistry = map[EngineName]engineCtor{}

// RegisterEngine registers the given engine name to an associated constructor.
func RegisterEngine(name EngineName, constructor engineCtor) {
	if _, old := engineRegistry[name]; old {
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

// ValidateSource validates the given source for the given engine. Typically
// this just means being able to parse/compile the code.
func ValidateSource(name EngineName, source string) error {
	engine, err := NewEngine(name)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(engine.Close)
	return engine.ValidateSource(source)
}
