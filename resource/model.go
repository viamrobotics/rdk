package resource

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

type (
	// ModelFamilyName is the model family.
	ModelFamilyName string

	// ModelName is the name of a specific model within a family.
	ModelName string
)

// DefaultModelFamilyName is the name "default".
const DefaultModelFamilyName = ModelFamilyName("builtin")

var (
	// DefaultModelFamily is the rdk:builtin model family for built-in resources.
	DefaultModelFamily        = ModelFamily{ResourceNamespaceRDK, DefaultModelFamilyName}
	modelRegexValidator       = regexp.MustCompile(`^([\w-]+):([\w-]+):([\w-]+)$`)
	singleFieldRegexValidator = regexp.MustCompile(`^([\w-]+)$`)
)

// ModelFamily is a family of related models.
type ModelFamily struct {
	Namespace Namespace       `json:"namespace"`
	Family    ModelFamilyName `json:"model_family"`
}

// NewModelFamily creates a new ModelFamily based on parameters passed in.
func NewModelFamily(namespace Namespace, family ModelFamilyName) ModelFamily {
	return ModelFamily{namespace, family}
}

// Validate ensures that important fields exist and are valid.
func (f ModelFamily) Validate() error {
	if f.Namespace == "" {
		return errors.New("namespace field for model missing")
	}
	if f.Family == "" {
		return errors.New("model_family field for model missing")
	}
	if err := ContainsReservedCharacter(string(f.Namespace)); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(string(f.Family)); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(f.Namespace)) {
		return errors.Errorf("string %q is not a valid model namespace", f.Namespace)
	}
	if !singleFieldRegexValidator.MatchString(string(f.Family)) {
		return errors.Errorf("string %q is not a valid model family", f.Family)
	}
	return nil
}

// String returns the model family string for the resource.
func (f ModelFamily) String() string {
	return fmt.Sprintf("%s:%s", f.Namespace, f.Family)
}

// Model represents an individual model within a family.
type Model struct {
	ModelFamily `json:","`
	Name        ModelName `json:"name"`
}

// NewModel creates a new Model based on parameters passed in.
func NewModel(namespace Namespace, fName ModelFamilyName, model ModelName) Model {
	family := NewModelFamily(namespace, fName)
	return Model{family, model}
}

// NewDefaultModel creates a new Model in the rdk:builtin namespace/family based on parameters passed in.
func NewDefaultModel(model ModelName) Model {
	return Model{DefaultModelFamily, model}
}

// NewModelFromString creates a new Name based on a fully qualified resource name string passed in.
func NewModelFromString(modelStr string) (Model, error) {
	if modelRegexValidator.MatchString(modelStr) {
		matches := modelRegexValidator.FindStringSubmatch(modelStr)
		return NewModel(Namespace(matches[1]), ModelFamilyName(matches[2]), ModelName(matches[3])), nil
	}

	// TODO(PRODUCT-266): Remove when triplet support complete
	if singleFieldRegexValidator.MatchString(modelStr) {
		return NewModel(ResourceNamespaceRDK, DefaultModelFamilyName, ModelName(modelStr)), nil
	}
	return Model{}, errors.Errorf("string %q is not a valid model name", modelStr)
}

// Validate ensures that important fields exist and are valid.
func (m Model) Validate() error {
	if err := m.ModelFamily.Validate(); err != nil {
		return err
	}
	if m.Name == "" {
		return errors.New("name field for model missing")
	}
	if err := ContainsReservedCharacter(string(m.Name)); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(m.Name)) {
		return errors.Errorf("string %q is not a valid model name", m.Name)
	}
	return nil
}

// String returns the resource model string for the component.
func (m Model) String() string {
	return fmt.Sprintf("%s:%s", m.ModelFamily, m.Name)
}

// UnmarshalJSON parses namespace:family:modelname strings to the full Model{} struct.
func (m *Model) UnmarshalJSON(data []byte) error {
	modelStr := strings.Trim(string(data), "\"'")
	if modelRegexValidator.MatchString(modelStr) {
		matches := modelRegexValidator.FindStringSubmatch(modelStr)
		m.Namespace = Namespace(matches[1])
		m.ModelFamily.Family = ModelFamilyName(matches[2])
		m.Name = ModelName(matches[3])
		return nil
	}

	// TODO(PRODUCT-266): Remove when triplet support complete
	if singleFieldRegexValidator.MatchString(modelStr) {
		m.Namespace = ResourceNamespaceRDK
		m.ModelFamily.Family = DefaultModelFamilyName
		m.Name = ModelName(modelStr)
		return nil
	}

	var tempModel map[string]string
	if err := json.Unmarshal(data, &tempModel); err != nil {
		return err
	}

	m.Namespace = Namespace(tempModel["namespace"])
	m.ModelFamily.Family = ModelFamilyName(tempModel["model_family"])
	m.Name = ModelName(tempModel["name"])

	return m.Validate()
}
