package resource

import (
	"reflect"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
)

// TODO(erd): rename these now that pkg is diff

// An AttributeMapConverter converts an attribute map into a possibly
// different representation.
// TODO(erd): does this go away.
type AttributeMapConverter func(attributes utils.AttributeMap) (interface{}, error)

// A AttributeMapConverterRegistration describes how to convert all attributes
// for a model of a type of resource.
type AttributeMapConverterRegistration struct {
	Subtype Subtype
	Model   Model
	Conv    AttributeMapConverter
}

// A AssociationConfigConverter describes how to convert all attributes
// for a type of resource associated with another resource (e.g. data capture on a resource).
type AssociationConfigConverter struct {
	Subtype  Subtype
	Conv     AttributeMapConverter
	WithName AssociationConfigWithName
}

// A AssocationConfigLinker describes how to associate a
// resource association config to a specific resource model (e.g. builtin data capture).
type AssocationConfigLinker struct {
	Subtype Subtype
	Model   Model
	Link    LinkAssocationConfig
}

type (
	// AssociationConfigWithName allows a resource to attach a name to a subtype specific
	// association config. This is generally done by the subtype registration.
	AssociationConfigWithName func(resName Name, resAssociation interface{}) error

	// LinkAssocationConfig allows one resource to associate a specific association config
	// to its own config. This is generally done by a specific resource (e.g. data capture of many components).
	LinkAssocationConfig func(conf *Config, resAssociation interface{}) error
)

var (
	attributeMapConverters         = []AttributeMapConverterRegistration{}
	associationConfigConverters    = []AssociationConfigConverter{}
	resourceAssocationConfigLinker = []AssocationConfigLinker{}
)

// RegisterComponentAttributeMapConverter associates a component type and model with a way to convert all attributes.
func RegisterComponentAttributeMapConverter(
	subtype Subtype,
	model Model,
	conv AttributeMapConverter,
) {
	RegisterAttributeMapConverter(subtype, model, conv)
}

// RegisterServiceAttributeMapConverter associates a service type and model with a way to convert all attributes.
// It is a helper for RegisterAttributeMapConverter.
func RegisterServiceAttributeMapConverter(
	subtype Subtype,
	model Model,
	conv AttributeMapConverter,
) {
	RegisterAttributeMapConverter(subtype, model, conv)
}

// RegisterAttributeMapConverter associates a resource (component/service) type and model with a way to
// convert all attributes.
func RegisterAttributeMapConverter(
	subtype Subtype,
	model Model,
	conv AttributeMapConverter,
) {
	attributeMapConverters = append(
		attributeMapConverters,
		AttributeMapConverterRegistration{subtype, model, conv},
	)
}

// TransformAttributeMap uses an attribute map to transform attributes to the prescribed format.
func TransformAttributeMap[T any](attributes utils.AttributeMap) (T, error) {
	var out T

	var forResult interface{}

	toT := reflect.TypeOf(out)
	if toT.Kind() == reflect.Ptr {
		// needs to be allocated then
		var ok bool
		out, ok = reflect.New(toT.Elem()).Interface().(T)
		if !ok {
			return out, errors.Errorf("failed to allocate default config type %T", out)
		}
		forResult = out
	} else {
		forResult = &out
	}

	var md mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:  "json",
		Result:   forResult,
		Metadata: &md,
	})
	if err != nil {
		return out, err
	}
	if err := decoder.Decode(attributes); err != nil {
		return out, err
	}
	if attributes.Has("attributes") || len(md.Unused) == 0 {
		return out, nil
	}
	// set as many unused attributes as possible
	toV := reflect.ValueOf(out)
	if toV.Kind() == reflect.Ptr {
		toV = toV.Elem()
	}
	if attrsV := toV.FieldByName("Attributes"); attrsV.IsValid() &&
		attrsV.Kind() == reflect.Map &&
		attrsV.Type().Key().Kind() == reflect.String {
		if attrsV.IsNil() {
			attrsV.Set(reflect.MakeMap(attrsV.Type()))
		}
		mapValueType := attrsV.Type().Elem()
		for _, key := range md.Unused {
			val := attributes[key]
			valV := reflect.ValueOf(val)
			if valV.Type().AssignableTo(mapValueType) {
				attrsV.SetMapIndex(reflect.ValueOf(key), valV)
			}
		}
	}
	return out, nil
}

// RegisterAssociationConfigConverter registers a converter for a resource's resource association config
// to the given resource subtype that will consume it (e.g. data capture on a component). Additionally, a way
// to attach a resource name to the converted config must be supplied.
func RegisterAssociationConfigConverter(
	subtype Subtype,
	conv AttributeMapConverter,
	withResourceName AssociationConfigWithName,
) {
	associationConfigConverters = append(
		associationConfigConverters,
		AssociationConfigConverter{subtype, conv, withResourceName},
	)
}

// RegisterAssocationConfigLinker registers a resource's association config type to a specific
// subtype model that will consume it (e.g. builtin data capture on a component).
func RegisterAssocationConfigLinker(
	subtype Subtype,
	model Model,
	link LinkAssocationConfig,
) {
	resourceAssocationConfigLinker = append(
		resourceAssocationConfigLinker,
		AssocationConfigLinker{subtype, model, link},
	)
}

// FindAssociationConfigConverter finds the resource association config AttributeMapConverter for the given subtype.
func FindAssociationConfigConverter(subtype Subtype) (AttributeMapConverter, AssociationConfigWithName, bool) {
	for _, r := range associationConfigConverters {
		if r.Subtype == subtype {
			return r.Conv, r.WithName, true
		}
	}
	return nil, nil, false
}

// FindAssocationConfigLinker finds the resource association to model associator for the given subtype and model.
func FindAssocationConfigLinker(subtype Subtype, model Model) (LinkAssocationConfig, bool) {
	for _, r := range resourceAssocationConfigLinker {
		if r.Subtype == subtype && r.Model == model {
			return r.Link, true
		}
	}
	return nil, false
}

// FindMapConverter finds the resource AttributeMapConverter for the given subtype and model.
// TODO(erd): rename.
func FindMapConverter(subtype Subtype, model Model) AttributeMapConverter {
	reg, ok := LookupRegistration(subtype, model)
	if !ok {
		return nil
	}
	return reg.AttributeMapConverter
}
