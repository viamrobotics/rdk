package mdl

func (f *Frame) GetGravity() (x, y, z float64) {
	return f.a.Linear.X(), f.a.Linear.Y(), f.a.Linear.Z()
}

func (f *Frame) SetGravity(x, y, z float64) {
	f.a.Linear[0] = x
	f.a.Linear[1] = y
	f.a.Linear[2] = z
}
