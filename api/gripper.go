package api

type Gripper interface {
	Open() error
	Grab() (bool, error)

	Close() error // closes the connection, not the gripper
}
