package config

// Module represents an external resource module, with path to a binary.
type Module struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Validate checks if the config is valid.
func (m *Module) Validate(path string) error {
	// TODO (@Otterverse)
	return nil
}
