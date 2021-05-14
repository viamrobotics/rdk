package referenceframe

type Position struct {
	X, Y, Z int64 // millimeters
}

type Rotation struct {
	Rx, Ry, Rz float64 // degrees
}

type Offset struct {
	Translation Position
	MyRotation  Rotation
}

func (o Offset) UnProject(p Position) Position {
	panic(1)
}

type Frame interface {
	Name() string
	NumChildren() int
	ChildFrame(n int) Frame
	ChildOffset(n int) Offset
}

func FindTranslation(from, to Frame) (Offset, error) {
	panic(1)
}

// ------

type basicFrame struct {
	name     string
	children []Frame
	offsets  []Offset
}

func (f *basicFrame) Name() string {
	return f.name
}

func (f *basicFrame) NumChildren() int {
	return len(f.children)
}

func (f *basicFrame) ChildFrame(n int) Frame {
	return f.children[n]
}

func (f *basicFrame) ChildOffset(n int) Offset {
	return f.offsets[n]
}

func (f *basicFrame) AddChild(newF Frame, off Offset) {
	f.children = append(f.children, newF)
	f.offsets = append(f.offsets, off)
}
