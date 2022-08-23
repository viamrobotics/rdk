package resource

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	// "github.com/jhump/protoreflect/desc".
	"github.com/pkg/errors"
)

type (
	// ModelFamilyName is the model family.
	ModelFamilyName string

	// ModelName is the name of a specific model within a family.
	ModelName string
)

// ModelFamilyDefaultName is the name "default".
const ModelFamilyDefaultName = ModelFamilyName("default")

var (
	// ModelFamilyDefault is the rdk:default model family for built-in resources.
	ModelFamilyDefault       = ModelFamily{ResourceNamespaceRDK, ModelFamilyDefaultName}
	modelRegexValidator      = regexp.MustCompile(`^([\w-]+):([\w-]+):([\w-]+)$`)
	shortModelRegexValidator = regexp.MustCompile(`^([\w-]+)$`)
)

// ModelFamily is a family of related models.
type ModelFamily struct {
	Namespace   Namespace `json:"namespace"`
	ModelFamily ModelFamilyName `json:"model_family"`
}

// NewModelFamily creates a new ModelFamily based on parameters passed in.
func NewModelFamily(namespace Namespace, family ModelFamilyName) ModelFamily {
	return ModelFamily{namespace, family}
}

// Validate ensures that important fields exist and are valid.
func (f ModelFamily) Validate() error {
	if f.Namespace == "" {
		return errors.New("model namespace field for resource missing")
	}
	if f.ModelFamily == "" {
		return errors.New("model family field for resource missing")
	}
	if err := ContainsReservedCharacter(string(f.Namespace)); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(string(f.ModelFamily)); err != nil {
		return err
	}
	return nil
}

// String returns the model family string for the resource.
func (f ModelFamily) String() string {
	return fmt.Sprintf("%s:%s", f.Namespace, f.ModelFamily)
}

// Model represents an individual model within a family.
type Model struct {
	ModelFamily `json:",squash"`
	Name ModelName `json:"name"`
}

// NewModel creates a new Model based on parameters passed in.
func NewModel(namespace Namespace, fName ModelFamilyName, model ModelName) Model {
	// fmt.Printf("SMURF901: %s, %s, %s\n", namespace, fName, model)
	family := NewModelFamily(namespace, fName)
	return Model{family, model}
}

// NewDefaultModel creates a new Model in the rdk:default namespace/family based on parameters passed in.
func NewDefaultModel(model ModelName) Model {
	return Model{ModelFamilyDefault, model}
}

// NewModelFromString creates a new Name based on a fully qualified resource name string passed in.
func NewModelFromString(modelStr string) (Model, error) {
	// fmt.Printf("SMURF900: %s\n", modelStr)
	if modelRegexValidator.MatchString(modelStr) {
		matches := modelRegexValidator.FindStringSubmatch(modelStr)
		// fmt.Printf("SMURF905: %s, %s, %s, %s\n", matches[0], matches[1], matches[2], matches[3])
		return NewModel(Namespace(matches[1]), ModelFamilyName(matches[2]), ModelName(matches[3])), nil
	}
	if shortModelRegexValidator.MatchString(modelStr) {
		return NewModel(ResourceNamespaceRDK, ModelFamilyDefaultName, ModelName(modelStr)), nil
	}
	return Model{}, errors.Errorf("string %q is not a valid model name", modelStr)
}

// Validate ensures that important fields exist and are valid.
func (m Model) Validate() error {
	if err := m.ModelFamily.Validate(); err != nil {
		return err
	}
	if m.Name == "" {
		return errors.New("model name field for resource missing")
	}
	if err := ContainsReservedCharacter(string(m.Name)); err != nil {
		return err
	}
	return nil
}

// String returns the resource model string for the component.
func (m Model) String() string {
	return fmt.Sprintf("%s:%s", m.ModelFamily, m.Name)
}

// UnmarshalJSON pareses namespace:family:modelname strings to the full Model{} struct.
func (m *Model) UnmarshalJSON(data []byte) error {
	modelStr := strings.Trim(string(data), "\"'")
	fmt.Printf("SMURF510: %s\n", modelStr)
	if modelRegexValidator.MatchString(modelStr) {
		matches := modelRegexValidator.FindStringSubmatch(modelStr)
		m.Namespace = Namespace(matches[1])
		m.ModelFamily.ModelFamily = ModelFamilyName(matches[2])
		m.Name = ModelName(matches[3])
		fmt.Printf("SMURF520: %+v\n", m)
		return nil
	}
	if shortModelRegexValidator.MatchString(modelStr) {
		m.Namespace = ResourceNamespaceRDK
		m.ModelFamily.ModelFamily = ModelFamilyDefaultName
		m.Name = ModelName(modelStr)
		fmt.Printf("SMURF521: %+v\n", m)
		return nil
	}

	var tempModel map[string]string
	err := json.Unmarshal(data, &tempModel)
	fmt.Printf("SMURF600: %+s decodes to %+v with error %v \n", data, tempModel, err)
	if err != nil {
		return err
	}

	m.Namespace = Namespace(tempModel["namespace"])
	m.ModelFamily.ModelFamily = ModelFamilyName(tempModel["model_family"])
	m.Name = ModelName(tempModel["name"])

	return nil
}
