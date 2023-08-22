package resource

import (
	"encoding/json"
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
	Name                      string
	API                       API
	Model                     Model
	Frame                     *referenceframe.LinkConfig
	DependsOn                 []string
	AssociatedResourceConfigs []AssociatedResourceConfig
	Attributes                utils.AttributeMap

	DiffingAttributes   ConfigValidator
	ConvertedAttributes ConfigValidator
	ImplicitDependsOn   []string

	alreadyValidated   bool
	cachedImplicitDeps []string
	cachedErr          error
}

// NOTE: This data must be maintained with what is in Config.
type typeSpecificConfigData struct {
	Name                      string                     `json:"name"`
	Namespace                 string                     `json:"namespace"`
	Subtype                   string                     `json:"type"`
	Model                     Model                      `json:"model"`
	Frame                     *referenceframe.LinkConfig `json:"frame,omitempty"`
	DependsOn                 []string                   `json:"depends_on,omitempty"`
	AssociatedResourceConfigs []AssociatedResourceConfig `json:"service_configs,omitempty"`
	Attributes                utils.AttributeMap         `json:"attributes,omitempty"`
}

// NOTE: This data must be maintained with what is in Config.
type configData struct {
	Name                      string                     `json:"name"`
	API                       API                        `json:"api"`
	Model                     Model                      `json:"model"`
	Frame                     *referenceframe.LinkConfig `json:"frame,omitempty"`
	DependsOn                 []string                   `json:"depends_on,omitempty"`
	AssociatedResourceConfigs []AssociatedResourceConfig `json:"service_configs,omitempty"`
	Attributes                utils.AttributeMap         `json:"attributes,omitempty"`
}

// UnmarshalJSON unmarshals JSON into the config.
func (conf *Config) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if _, ok := m["api"]; ok {
		var confData configData
		if err := json.Unmarshal(data, &confData); err != nil {
			return err
		}
		conf.Name = confData.Name
		conf.API = confData.API
		conf.Model = confData.Model
		conf.Frame = confData.Frame
		conf.DependsOn = confData.DependsOn
		conf.AssociatedResourceConfigs = confData.AssociatedResourceConfigs
		conf.Attributes = confData.Attributes
		return nil
	}

	var typeSpecificConf typeSpecificConfigData
	if err := json.Unmarshal(data, &typeSpecificConf); err != nil {
		return err
	}
	conf.Name = typeSpecificConf.Name
	// this will get adjusted later
	conf.API = APINamespace(typeSpecificConf.Namespace).WithType("").WithSubtype(typeSpecificConf.Subtype)
	conf.Model = typeSpecificConf.Model
	conf.Frame = typeSpecificConf.Frame
	conf.DependsOn = typeSpecificConf.DependsOn
	conf.AssociatedResourceConfigs = typeSpecificConf.AssociatedResourceConfigs
	conf.Attributes = typeSpecificConf.Attributes
	return nil
}

// MarshalJSON marshals JSON from the config.
func (conf Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(configData{
		Name:                      conf.Name,
		API:                       conf.API,
		Model:                     conf.Model,
		Frame:                     conf.Frame,
		DependsOn:                 conf.DependsOn,
		AssociatedResourceConfigs: conf.AssociatedResourceConfigs,
		Attributes:                conf.Attributes,
	})
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
		Name:  name.Name,
		API:   name.API,
		Model: model,
	}
}

// An AssociatedResourceConfig describes configuration of a resource for an associated resource.
type AssociatedResourceConfig struct {
	API                 API
	Attributes          utils.AttributeMap
	ConvertedAttributes interface{}
	RemoteName          string
}

// NOTE: This data must be maintained with what is in AssociatedResourceConfig.
type associatedResourceConfigData struct {
	API        string             `json:"type"`
	Attributes utils.AttributeMap `json:"attributes"`
}

// UnmarshalJSON unmarshals JSON into the config.
func (assoc *AssociatedResourceConfig) UnmarshalJSON(data []byte) error {
	var confData associatedResourceConfigData
	if err := json.Unmarshal(data, &confData); err != nil {
		return err
	}

	assoc.Attributes = confData.Attributes

	api, err := NewPossibleRDKServiceAPIFromString(confData.API)
	if err != nil {
		return err
	}
	assoc.API = api
	return nil
}

// MarshalJSON marshals JSON from the config.
func (assoc AssociatedResourceConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(associatedResourceConfigData{
		API:        assoc.API.String(),
		Attributes: assoc.Attributes,
	})
}

// Equals checks if the two configs are logically equivalent. From the perspective of the
// component's JSON configuration string.
func (conf Config) Equals(other Config) bool {
	//nolint:govet
	switch {
	case conf.Name != other.Name:
		return false
	case conf.API != other.API:
		return false
	case conf.Model != other.Model:
		return false
	case !reflect.DeepEqual(conf.DependsOn, other.DependsOn):
		return false
	case !reflect.DeepEqual(conf.AssociatedResourceConfigs, other.AssociatedResourceConfigs):
		return false
	case !reflect.DeepEqual(conf.Attributes, other.Attributes):
		// Only one of `Attributes` or `DiffingAttributes` should be in populated.
		return false
	case !reflect.DeepEqual(conf.DiffingAttributes, other.DiffingAttributes):
		return false
	}

	return true
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
		rName := NewName(conf.API, remotes[len(remotes)-1])
		return rName.PrependRemote(strings.Join(remotes[:len(remotes)-1], ":"))
	}
	return NewName(conf.API, conf.Name)
}

// Validate ensures all parts of the config are valid and returns dependencies.
func (conf *Config) Validate(path, defaultAPIType string) ([]string, error) {
	if conf.alreadyValidated {
		return conf.cachedImplicitDeps, conf.cachedErr
	}
	conf.cachedImplicitDeps, conf.cachedErr = conf.validate(path, defaultAPIType)
	conf.alreadyValidated = true
	return conf.cachedImplicitDeps, conf.cachedErr
}

// AdjustPartialNames assumes this config comes from a place where the resource
// name, API names, Model names, and associated config type names are partially
// stored (JSON/Proto/Database) and will fix them up to the builtin values they
// are intended for.
func (conf *Config) AdjustPartialNames(defaultAPIType string) {
	if conf.API.Type.Namespace == "" {
		conf.API.Type.Namespace = APINamespaceRDK
	}
	if conf.API.Type.Name == "" {
		conf.API.Type.Name = defaultAPIType
	}

	if conf.Model.Family.Namespace == "" {
		conf.Model.Family.Namespace = DefaultModelFamily.Namespace
	}
	if conf.Model.Family.Name == "" {
		conf.Model.Family.Name = DefaultModelFamily.Name
	}

	if conf.API.IsService() {
		// If services do not have names use the name builtin
		if conf.Name == "" {
			conf.Name = DefaultServiceName
		}
		if conf.Model.Name == "" {
			conf.Model = DefaultServiceModel
		}
	}
}

func (conf *Config) validate(path, defaultAPIType string) ([]string, error) {
	var deps []string

	conf.AdjustPartialNames(defaultAPIType)

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
