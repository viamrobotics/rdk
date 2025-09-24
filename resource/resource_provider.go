package resource

// Provider is a generic interface for looking up resources by name.
type Provider[T Resource] interface {
	GetResource(src any, name string) (T, error)
}
