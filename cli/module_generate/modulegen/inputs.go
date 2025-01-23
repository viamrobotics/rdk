// Package modulegen contains defined types used for module generation
package modulegen

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ModuleInputs contains the necessary information to fill out template files.
type ModuleInputs struct {
	ModuleName       string    `json:"module_name"`
	IsPublic         bool      `json:"-"`
	Namespace        string    `json:"namespace"`
	OrgID            string    `json:"-"`
	Language         string    `json:"language"`
	Resource         string    `json:"-"`
	ResourceType     string    `json:"resource_type"`
	ResourceSubtype  string    `json:"resource_subtype"`
	ModelName        string    `json:"model_name"`
	EnableCloudBuild bool      `json:"enable_cloud_build"`
	InitializeGit    bool      `json:"initialize_git"`
	RegisterOnApp    bool      `json:"-"`
	GeneratorVersion string    `json:"generator_version"`
	GeneratedOn      time.Time `json:"generated_on"`

	ModulePascal          string `json:"-"`
	ModuleCamel           string `json:"-"`
	ModuleLowercase       string `json:"-"`
	API                   string `json:"-"`
	ResourceSubtypePascal string `json:"-"`
	ResourceTypePascal    string `json:"-"`
	ModelPascal           string `json:"-"`
	ModelCamel            string `json:"-"`
	ModelTriple           string `json:"-"`
	ModelLowercase        string `json:"-"`

	SDKVersion string `json:"-"`
}

// Resources is a list of all the available resources in Viam.
var Resources = []string{
	"arm component",
	"audio_input component",
	"base component",
	"board component",
	"camera component",
	"encoder component",
	"gantry component",
	"generic_component component",
	"gripper component",
	"input component",
	"motor component",
	"movement_sensor component",
	"pose_tracker component",
	"power_sensor component",
	"sensor component",
	"servo component",
	"generic_service service",
	"mlmodel service",
	"motion service",
	"navigation service",
	"slam service",
	"vision service",
}

// GoModuleTmpl contains necessary information to fill out the go method stubs.
type GoModuleTmpl struct {
	Module    ModuleInputs
	ModelType string
	ObjName   string
	Imports   string
	Functions string
}

// HasEmptyInput checks to see if any required inputs were not filled in.
func (inputs *ModuleInputs) HasEmptyInput() bool {
	requiredInputs := []string{
		inputs.ModuleName, inputs.Language, inputs.Namespace, inputs.ResourceSubtype, inputs.ModelName,
	}
	for _, input := range requiredInputs {
		if input == "" {
			return true
		}
	}
	return false
}

// SpaceReplacer removes spaces, dashes, and underscores from a string.
var SpaceReplacer = strings.NewReplacer(" ", "", "_", "", "-", "")

// CheckResourceAndSetType checks if the given resource subtype is valid, and sets the corresponding resource type if so.
func (inputs *ModuleInputs) CheckResourceAndSetType() error {
	if inputs.ResourceSubtype == "" {
		return nil
	}
	if inputs.ResourceSubtype == "generic" {
		return errors.New(
			"resource subtype 'generic' cannot be differentiated; please specify either 'generic-service' or 'generic-component'")
	}
	for _, resource := range Resources {
		splitResource := strings.Split(resource, " ")
		if len(splitResource) != 2 {
			return errors.New("resource not formatted correctly in code base; this shouldn't happen. please consider filing a ticket")
		}
		subtype := splitResource[0]
		// make sure we support subtypes that are passed with different spacers (e.g.,
		// "power sensor", "power-sensor", "power_sensor", "powerSensor")
		if strings.ToLower(SpaceReplacer.Replace(inputs.ResourceSubtype)) == SpaceReplacer.Replace(subtype) {
			// we need users to specify if a generic is a component or service, but we want to
			// internally simplify the subtype back down to `generic`
			if subtype == "generic_component" || subtype == "generic_service" {
				inputs.ResourceSubtype = "generic"
			} else {
				// use the canonically correct subtype formatting so we don't have to continue
				// supporting all variations of it everywhere in the codebase
				inputs.ResourceSubtype = subtype
			}
			inputs.ResourceType = splitResource[1]
			inputs.Resource = inputs.ResourceSubtype + " " + inputs.ResourceType
			return nil
		}
	}
	return fmt.Errorf("given resource subtype '%s' does not exist", inputs.ResourceSubtype)
}
