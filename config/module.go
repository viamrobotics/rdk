package config

import (
	"go.viam.com/rdk/resource"
)

type Module struct {
	Name   string           `json:"name"`
	Path   string           `json:"path"`
	Type   string           `json:"type"`
	Models []resource.Model `json:"models"`
}

func (m *Module) Validate(path string) error {
	// TODO
	return nil
}

func (m *Module) ResourceName() string {
	return m.Name
}
