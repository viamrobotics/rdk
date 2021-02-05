package sensor

type Device interface {
	Readings() ([]interface{}, error)
	Close() error
}
