package resource

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
)

// ModelNamespaceRDK is the namespace to use for models implemented by the rdk.
const ModelNamespaceRDK = ModelNamespace("rdk")

var (
	// DefaultModelFamily is the rdk:builtin model family for built-in resources.
	DefaultModelFamily = ModelNamespaceRDK.WithFamily("builtin")
	// DefaultServiceModel is used for builtin services.
	DefaultServiceModel = DefaultModelFamily.WithModel("builtin")
	modelRegexValidator = regexp.MustCompile(`^([\w-]+):([\w-]+):([\w-]+)$`)

	// this is used for the legacy style model name at the lowest level (e.g. rdk:builtin:->builtin<-).
	singleFieldRegexValidator = regexp.MustCompile(`^([\w-]+)$`)
)

// Model represents an individual model within a family.
// It consists of a Namespace, Family, and Name.
type Model struct {
	Family ModelFamily `json:","`
	Name   string      `json:"name"`
}

// ModelFamily is a family of related models.
type ModelFamily struct {
	Namespace ModelNamespace `json:"namespace"`
	Name      string         `json:"model_family"`
}

// WithModel returns a new model with the given name.
func (f ModelFamily) WithModel(name string) Model {
	return Model{f, name}
}

// ModelNamespace identifies the namespaces resource models can live in.
type ModelNamespace string

// WithFamily returns a new model family with the given name.
func (n ModelNamespace) WithFamily(name string) ModelFamily {
	return ModelFamily{n, name}
}

// Validate ensures that important fields exist and are valid.
func (f ModelFamily) Validate() error {
	if f.Namespace == "" {
		return errors.New("namespace field for model missing")
	}
	if f.Name == "" {
		return errors.New("model_family field for model missing")
	}
	if err := ContainsReservedCharacter(string(f.Namespace)); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(f.Name); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(f.Namespace)) {
		return errors.Errorf("string %q is not a valid model namespace", f.Namespace)
	}
	if !singleFieldRegexValidator.MatchString(f.Name) {
		return errors.Errorf("string %q is not a valid model family", f.Name)
	}
	return nil
}

// String returns the model family string for the resource.
func (f ModelFamily) String() string {
	return fmt.Sprintf("%s:%s", f.Namespace, f.Name)
}

// NewModel return a new model from a triplet like acme:demo:mygizmo.
func NewModel(namespace, family, modelName string) Model {
	return ModelNamespace(namespace).WithFamily(family).WithModel(modelName)
}

// NewModelFamily returns a new family from the given namespace and family.
func NewModelFamily(namespace, family string) ModelFamily {
	return ModelNamespace(namespace).WithFamily(family)
}

// NewModelFromString creates a new Name based on a fully qualified resource name string passed in.
func NewModelFromString(modelStr string) (Model, error) {
	if modelRegexValidator.MatchString(modelStr) {
		matches := modelRegexValidator.FindStringSubmatch(modelStr)
		return ModelNamespace(matches[1]).WithFamily(matches[2]).WithModel(matches[3]), nil
	}

	if singleFieldRegexValidator.MatchString(modelStr) {
		return DefaultModelFamily.WithModel(modelStr), nil
	}
	return Model{}, errors.Errorf("string %q is not a valid model name", modelStr)
}

// Validate ensures that important fields exist and are valid.
func (m Model) Validate() error {
	if err := m.Family.Validate(); err != nil {
		return err
	}
	if m.Name == "" {
		return errors.New("name field for model missing")
	}
	if err := ContainsReservedCharacter(m.Name); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(m.Name) {
		return errors.Errorf("string %q is not a valid model name", m.Name)
	}
	return nil
}

// String returns the resource model name in its triplet form.
func (m Model) String() string {
	return fmt.Sprintf("%s:%s:%s", m.Family.Namespace, m.Family.Name, m.Name)
}

// MarshalJSON marshals the model name in its triplet form.
func (m Model) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.String())
}

// UnmarshalJSON parses namespace:family:modelname strings to the full Model{} struct.
func (m *Model) UnmarshalJSON(data []byte) error {
	var modelStr string
	if err := json.Unmarshal(data, &modelStr); err == nil {
		switch {
		case modelRegexValidator.MatchString(modelStr):
			matches := modelRegexValidator.FindStringSubmatch(modelStr)
			*m = ModelNamespace(matches[1]).WithFamily(matches[2]).WithModel(matches[3])
			return nil
		case singleFieldRegexValidator.MatchString(modelStr):
			*m = DefaultModelFamily.WithModel(modelStr)
			return nil
		default:
			return fmt.Errorf("not a valid Model config string. Input: `%v`", modelStr)
		}
	}

	var tempModel map[string]string
	if err := json.Unmarshal(data, &tempModel); err != nil {
		return errors.Wrapf(err,
			"%q is not a valid model. "+
				`models must be of the form "namespace:family:name", "name", or be valid nested JSON `+
				`with "namespace", "model_family" and "name" fields`, modelStr)
	}

	m.Family.Namespace = ModelNamespace(tempModel["namespace"])
	m.Family.Name = tempModel["model_family"]
	m.Name = tempModel["name"]

	return m.Validate()
}
