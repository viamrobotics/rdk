package config

type Module struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (m *Module) Validate(path string) error {
	// TODO
	return nil
}

func (m *Module) ResourceName() string {
	return m.Name
}
