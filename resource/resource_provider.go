package resource

// Provider defines an interface for looking up resources by their name.
type Provider interface {
	// GetResource returns the Resource associated with the given name.
	GetResource(name Name) (Resource, error)
}
