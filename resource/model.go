package resource

import (
	// "context"
	"fmt"
	"regexp"
	// "strings"

	// "github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
)

type (
	// ModelFamilyName is the model family
	ModelFamilyName string

	// ModelName is the name of a specific model within a family
	ModelName string
)

const ModelFamilyDefaultName = ModelFamilyName("default")

var (
	ModelFamilyDefault = ModelFamily{ResourceNamespaceRDK, ModelFamilyDefaultName}
	modelRegexValidator = regexp.MustCompile(`^(\w+):(\w+):(\w+)$`)
	shortModelRegexValidator = regexp.MustCompile(`^(\w+)$`)
)


// ModelFamily is a family of related models.
type ModelFamily struct {
	Namespace Namespace
	ModelFamily ModelFamilyName
}

// NewModelFamily creates a new ModelFamily based on parameters passed in.
func NewModelFamily(namespace Namespace, family ModelFamilyName) ModelFamily {
	return ModelFamily{namespace, family}
}

// Validate ensures that important fields exist and are valid.
func (f ModelFamily) Validate() error {
	if f.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid")
	}
	if f.ModelFamily == "" {
		return errors.New("modelfamily field for resource missing or invalid")
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
	ModelFamily
	Name ModelName
}

// NewModel creates a new Model based on parameters passed in.
func NewModel(namespace Namespace, fName ModelFamilyName, model ModelName) Model {
	family := NewModelFamily(namespace, fName)
	return Model{family, model}
}

// NewModelFromString creates a new Name based on a fully qualified resource name string passed in.
func NewModelFromString(modelStr string) (Model, error) {
	if modelRegexValidator.MatchString(modelStr) {
		matches := resRegexValidator.FindStringSubmatch(modelStr)
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
		return errors.New("model name field for resource missing or invalid")
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
