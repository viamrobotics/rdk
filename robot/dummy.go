package robot

type dummyGripper struct {
}

func (dg *dummyGripper) Open() error {
	return nil
}

func (dg *dummyGripper) Close() error {
	return nil
}

func (dg *dummyGripper) Grab() (bool, error) {
	return false, nil
}
