// Package common contains defined types used for module generation
package common

import "time"

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
	requiredInputs := []string{inputs.ModuleName, inputs.Language, inputs.Namespace, inputs.ResourceType, inputs.ResourceSubtype, inputs.ModelName}
	for _, input := range requiredInputs {
		if input == "" {
			return true
		}
	}
	return false
}
