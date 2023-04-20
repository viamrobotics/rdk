package resource

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// A Config describes the configuration of a resource.
type Config struct {
	Name string `json:"name"`

	// TODO(PRODUCT-266): API replaces Type and Namespace when Service/Component merge, so json needs to be enabled.
	DeprecatedNamespace    Namespace   `json:"namespace"`
	DeprecatedSubtype      SubtypeName `json:"type"`
	DeprecatedResourceType TypeName    `json:"-"`
	API                    Subtype     `json:"-"`

	Model     Model                      `json:"model"`
	Frame     *referenceframe.LinkConfig `json:"frame,omitempty"`
	DependsOn []string                   `json:"depends_on"`
	// could be components in the future but right now its services.
	AssociatedResourceConfigs []AssociatedResourceConfig `json:"service_configs"`

	Attributes          utils.AttributeMap `json:"attributes"`
	ConvertedAttributes ConfigValidator    `json:"-"`
	ImplicitDependsOn   []string           `json:"-"`

	alreadyValidated   bool
	cachedImplicitDeps []string
	cachedErr          error
}

// NativeConfig returns the native config from the given config via its
// converted attributes. When generics are better in go to support a mapping
// of Models -> T's (cannot right now because of type instantiation rules), then
// this should be a method on the type and hide away both Attributes and
// ConvertedAttributes.
func NativeConfig[T any](conf Config) (T, error) {
	return utils.AssertType[T](conf.ConvertedAttributes)
}

// NewEmptyConfig returns a new, empty config for the given name and model.
func NewEmptyConfig(name Name, model Model) Config {
	return Config{
		Name:                   name.Name,
		DeprecatedNamespace:    name.Namespace,
		DeprecatedSubtype:      name.ResourceSubtype,
		DeprecatedResourceType: name.ResourceType,
		API:                    name.Subtype,
		Model:                  model,
	}
}

// An AssociatedResourceConfig describes configuration of a resource for an associated resource.
type AssociatedResourceConfig struct {
	Type                SubtypeName        `json:"type"`
	Attributes          utils.AttributeMap `json:"attributes"`
	ConvertedAttributes interface{}        `json:"-"`
}

// AssociatedSubtype returns the subtype that this config is associated with.
func (conf *AssociatedResourceConfig) AssociatedSubtype() Subtype {
	cType := string(conf.Type)
	return NewSubtype(
		ResourceNamespaceRDK,
		ResourceTypeService,
		SubtypeName(cType),
	)
}

// Equals checks if the two configs are deeply equal to each other.
func (conf Config) Equals(other Config) bool {
	conf.alreadyValidated = false
	conf.cachedImplicitDeps = nil
	conf.cachedErr = nil
	other.alreadyValidated = false
	other.cachedImplicitDeps = nil
	other.cachedErr = nil
	//nolint:govet
	return reflect.DeepEqual(conf, other)
}

// Dependencies returns the deduplicated union of user-defined and implicit dependencies.
func (conf *Config) Dependencies() []string {
	result := make([]string, 0, len(conf.DependsOn)+len(conf.ImplicitDependsOn))
	seen := make(map[string]struct{})
	appendUniq := func(dep string) {
		if _, ok := seen[dep]; !ok {
			seen[dep] = struct{}{}
			result = append(result, dep)
		}
	}
	for _, dep := range conf.DependsOn {
		appendUniq(dep)
	}
	for _, dep := range conf.ImplicitDependsOn {
		appendUniq(dep)
	}
	return result
}

// String returns a verbose representation of the config.
func (conf *Config) String() string {
	return fmt.Sprintf("%#v", conf)
}

// ResourceName returns the  ResourceName for the component.
func (conf *Config) ResourceName() Name {
	remotes := strings.Split(conf.Name, ":")
	if len(remotes) > 1 {
		rName := NameFromSubtype(conf.API, remotes[len(remotes)-1])
		return rName.PrependRemote(RemoteName(strings.Join(remotes[:len(remotes)-1], ":")))
	}
	return NameFromSubtype(conf.API, conf.Name)
}

// Validate ensures all parts of the config are valid and returns dependencies.
func (conf *Config) Validate(path string, defaultType TypeName) ([]string, error) {
	if conf.alreadyValidated {
		return conf.cachedImplicitDeps, conf.cachedErr
	}
	conf.cachedImplicitDeps, conf.cachedErr = conf.validate(path, defaultType)
	conf.alreadyValidated = true
	return conf.cachedImplicitDeps, conf.cachedErr
}

func (conf *Config) validate(path string, defaultType TypeName) ([]string, error) {
	var deps []string

	//nolint:gocritic
	if conf.API.Namespace == "" && conf.DeprecatedNamespace == "" {
		conf.DeprecatedNamespace = ResourceNamespaceRDK
		conf.API.Namespace = conf.DeprecatedNamespace
	} else if conf.API.Namespace == "" {
		conf.API.Namespace = conf.DeprecatedNamespace
	} else {
		conf.DeprecatedNamespace = conf.API.Namespace
	}

	if conf.API.ResourceType == "" {
		conf.API.ResourceType = defaultType
	}

	if conf.API.ResourceSubtype == "" {
		conf.API.ResourceSubtype = conf.DeprecatedSubtype
	} else if conf.DeprecatedSubtype == "" {
		conf.DeprecatedSubtype = conf.API.ResourceSubtype
	}

	if conf.API.Namespace != conf.DeprecatedNamespace ||
		conf.API.ResourceType != conf.DeprecatedResourceType ||
		conf.API.ResourceSubtype != conf.DeprecatedSubtype {
		// ignore already set namespace and type
		conf.DeprecatedNamespace = conf.API.Namespace
		conf.DeprecatedResourceType = conf.API.ResourceType
		conf.DeprecatedSubtype = conf.API.ResourceSubtype
	}

	if conf.DeprecatedNamespace == "" {
		// NOTE: This should never be removed in order to ensure RDK is the
		// default namespace.
		conf.DeprecatedNamespace = ResourceNamespaceRDK
	}

	if conf.Model.Namespace == "" {
		conf.Model.Namespace = ResourceNamespaceRDK
		conf.Model.ModelFamily = DefaultModelFamily
	}

	if defaultType == ResourceTypeService {
		// If services do not have names use the name builtin
		if conf.Name == "" {
			conf.Name = DefaultServiceName
		}
		if conf.Model.Name == "" {
			conf.Model = DefaultServiceModel
		}
	}

	if conf.Name == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if !utils.ValidNameRegex.MatchString(conf.Name) {
		return nil, utils.ErrInvalidName(conf.Name)
	}
	if err := ContainsReservedCharacter(conf.Name); err != nil {
		return nil, err
	}

	if err := conf.Model.Validate(); err != nil {
		return nil, err
	}

	// this effectively checks reserved characters and the rest for namespace and type
	if err := conf.API.Validate(); err != nil {
		return nil, err
	}
	if conf.ConvertedAttributes != nil {
		validatedDeps, err := conf.ConvertedAttributes.Validate(path)
		if err != nil {
			return nil, err
		}
		deps = append(deps, validatedDeps...)
	}
	return deps, nil
}

// A ConfigValidator validates a configuration and also
// returns dependencies that were implicitly discovered.
type ConfigValidator interface {
	Validate(path string) ([]string, error)
}

// TransformAttributeMap uses an attribute map to transform attributes to the prescribed format.
func TransformAttributeMap[T any](attributes utils.AttributeMap) (T, error) {
	var out T

	var forResult interface{}

	toT := reflect.TypeOf(out)
	if toT == nil {
		// nothing to transform
		return out, nil
	}
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
