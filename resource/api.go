package resource

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
)

const (
	// APINamespaceRDK is the namespace to use for APIs defined by the standard robot API.
	APINamespaceRDK = APINamespace("rdk")

	// APINamespaceRDKInternal is the namespace to use for internal services.
	APINamespaceRDKInternal = APINamespace("rdk-internal")

	// APITypeServiceName is for any service in any namespace.
	APITypeServiceName = "service"

	// APITypeComponentName is for any component in any namespace.
	APITypeComponentName = "component"
)

var apiRegexValidator = regexp.MustCompile(`^([\w-]+):([\w-]+):([\w-]+)$`)

// API represents a known component/service (resource) API.
// It consists of a Namespace, Type, and API.
type API struct {
	Type        APIType
	SubtypeName string `json:"subtype"`
}

// IsComponent returns if this API is for a component.
func (a API) IsComponent() bool {
	return a.Type.Name == APITypeComponentName
}

// IsService returns if this API is for a service.
func (a API) IsService() bool {
	return a.Type.Name == APITypeServiceName
}

// APIType represents a known component/service type of a robot.
type APIType struct {
	Namespace APINamespace `json:"namespace"`
	Name      string       `json:"type"`
}

// WithSubtype returns an API with the given subtype name.
func (t APIType) WithSubtype(subtypeName string) API {
	return API{t, subtypeName}
}

// APINamespace identifies the namespaces robot resources can live in.
type APINamespace string

// WithType returns an API Type with the given name.
func (n APINamespace) WithType(name string) APIType {
	return APIType{n, name}
}

// WithComponentType returns an API with the given component name.
func (n APINamespace) WithComponentType(subtypeName string) API {
	return n.WithType(APITypeComponentName).WithSubtype(subtypeName)
}

// WithServiceType returns an API with the given service name.
func (n APINamespace) WithServiceType(subtypeName string) API {
	return n.WithType(APITypeServiceName).WithSubtype(subtypeName)
}

// Validate ensures that important fields exist and are valid.
func (t APIType) Validate() error {
	if t.Namespace == "" {
		return errors.New("namespace field for resource missing or invalid")
	}
	if t.Name == "" {
		return errors.New("type field for resource missing or invalid")
	}
	if err := ContainsReservedCharacter(string(t.Namespace)); err != nil {
		return err
	}
	if err := ContainsReservedCharacter(t.Name); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(string(t.Namespace)) {
		return errors.Errorf("string %q is not a valid type namespace", t.Namespace)
	}
	if !singleFieldRegexValidator.MatchString(t.Name) {
		return errors.Errorf("string %q is not a valid type name", t.Name)
	}
	return nil
}

// String returns the resource type string for the component.
func (t APIType) String() string {
	return fmt.Sprintf("%s:%s", t.Namespace, t.Name)
}

// Validate ensures that important fields exist and are valid.
func (a API) Validate() error {
	if err := a.Type.Validate(); err != nil {
		return err
	}
	if a.SubtypeName == "" {
		return errors.New("subtype field for resource missing or invalid")
	}
	if err := ContainsReservedCharacter(a.SubtypeName); err != nil {
		return err
	}
	if !singleFieldRegexValidator.MatchString(a.SubtypeName) {
		return errors.Errorf("string %q is not a valid subtype name", a.SubtypeName)
	}
	return nil
}

// String returns the triplet form of the API name.
func (a API) String() string {
	return fmt.Sprintf("%s:%s:%s", a.Type.Namespace, a.Type.Name, a.SubtypeName)
}

// MarshalJSON marshals the API name in its triplet form.
func (a API) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON parses either a string of the form namespace:type:subtype or a json object into an
// API object.
func (a *API) UnmarshalJSON(data []byte) error {
	var apiStr string
	if err := json.Unmarshal(data, &apiStr); err == nil {
		// If the value is a string, regex match for a colon partitioned triplet.
		if !apiRegexValidator.MatchString(apiStr) {
			return fmt.Errorf("not a valid API config string. Input: `%v`", string(data))
		}

		matches := apiRegexValidator.FindStringSubmatch(apiStr)
		*a = APINamespace(matches[1]).WithType(matches[2]).WithSubtype(matches[3])
		return nil
	}

	var tempSt map[string]string
	if err := json.Unmarshal(data, &tempSt); err != nil {
		return fmt.Errorf("API config value is neither a string nor JSON object. Input: %v", string(data))
	}

	*a = APINamespace(tempSt["namespace"]).WithType(tempSt["type"]).WithSubtype(tempSt["subtype"])
	return a.Validate()
}

// NewAPI return a new API from a triplet like acme:component:gizmo.
func NewAPI(namespace, typeName, subtypeName string) API {
	return APINamespace(namespace).WithType(typeName).WithSubtype(subtypeName)
}

// NewAPIFromString creates a new API from a fully qualified string in the format namespace:type:subtype.
func NewAPIFromString(apiStr string) (API, error) {
	if !apiRegexValidator.MatchString(apiStr) {
		return API{}, errors.Errorf("string %q is not a valid api name", apiStr)
	}
	matches := apiRegexValidator.FindStringSubmatch(apiStr)
	return APINamespace(matches[1]).WithType(matches[2]).WithSubtype(matches[3]), nil
}

// NewPossibleRDKServiceAPIFromString returns an API from a string that if is a singular name,
// will be interpreted as an RDK service.
func NewPossibleRDKServiceAPIFromString(apiStr string) (API, error) {
	api, apiErr := NewAPIFromString(apiStr)
	if apiErr == nil {
		return api, nil
	}
	if !singleFieldRegexValidator.MatchString(apiStr) {
		return API{}, apiErr
	}

	// assume it is a builtin service
	return APINamespaceRDK.WithServiceType(apiStr), nil
}
