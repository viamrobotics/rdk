// Package mythingamabob implements an acme:component:thingamabob, a component that demonstrates custom Validation logic.
package mythingamabob

import (
	"context"
	"errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/thingamabobapi"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel(
	resource.Namespace("acme"),
	resource.ModelFamilyName("demo"),
	resource.ModelName("mythingamabob"),
)

// ThingamabobConfig is how you configure a thingamabob. All thingamabobs depend on
// a gizmo.
type ThingamabobConfig struct {
	Gizmo string `json:"gizmo"`
}

func (cfg *ThingamabobConfig) Validate(path string) ([]string, error) {
	if cfg.Gizmo == "" {
		return nil, errors.New("expected 'gizmo' attribute for thingamabob")
	}

	return []string{cfg.Gizmo}, nil
}

func init() {
	registry.RegisterComponent(thingamabobapi.Subtype, Model, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewMyThingamabob(deps, config, logger)
		},
	})

	// Use RegisterComponentAttributeMapConverter to register a custom configuration
	// struct that has a Validate(string) ([]string, error) method.
	//
	// The Validate method will automatically be called in RDK's module manager to
	// Validate the Thingamabob's configuration and register implicit dependencies.
	config.RegisterComponentAttributeMapConverter(
		thingamabobapi.Subtype,
		Model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf ThingamabobConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&ThingamabobConfig{})
}

type myActualThingamabob struct {
	gizmo string
}

func NewMyThingamabob(
	deps registry.Dependencies,
	conf config.Component,
	logger golog.Logger,
) (thingamabobapi.Thingamabob, error) {
	cfg, ok := conf.ConvertedAttributes.(*ThingamabobConfig)
	if !ok {
		return nil, errors.New("unrecognized attributes for thingamabob")
	}
	return &myActualThingamabob{gizmo: cfg.Gizmo}, nil
}
