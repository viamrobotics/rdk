package gripper

type Gripper interface {
	Open() error
	Close() error
	Grab() (bool, error)
}
