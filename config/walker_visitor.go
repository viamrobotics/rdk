package config

// Walker is a portion of the config that can be walked.
type Walker interface {
	// Walk walks a structure and returns a new structure, with or without modifications.
	Walk(Visitor) (interface{}, error)
}

// Visitor defines an interface for visiting and potentially modifying portions of the config.
type Visitor interface {
	// Visit visits a node and returns a new node, with or without modifications.
	Visit(interface{}) (interface{}, error)
}
