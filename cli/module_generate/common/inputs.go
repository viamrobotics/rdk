// Package common contains defined types used for module generation
package common

import (
	"fmt"
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
		inputs.ModuleName, inputs.Language, inputs.Namespace, inputs.ResourceType, inputs.ResourceSubtype, inputs.ModelName,
	}
	for _, input := range requiredInputs {
		if input == "" {
			return true
		}
	}
	return false
}

// CheckResource checks if the given resource is valid.
func (inputs *ModuleInputs) CheckResource() error {
	resources := []string{
		"arm component",
		"audio_input component",
		"base component",
		"board component",
		"camera component",
		"encoder component",
		"gantry component",
		"generic component",
		"gripper component",
		"input component",
		"motor component",
		"movement_sensor component",
		"pose_tracker component",
		"power_sensor component",
		"sensor component",
		"servo component",
		"generic service",
		"mlmodel service",
		"motion service",
		"navigation service",
		"slam service",
		"vision service",
	}
	for _, resource := range resources {
		if inputs.Resource == resource {
			return nil
		}
	}
	return fmt.Errorf("given resource '%s' does not exist", inputs.Resource)
}
