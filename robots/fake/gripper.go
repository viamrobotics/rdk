package fake

type Gripper struct {
}

func (g *Gripper) Open() error {
	return nil
}

func (g *Gripper) Close() error {
	return nil
}

func (g *Gripper) Grab() (bool, error) {
	return false, nil
}
