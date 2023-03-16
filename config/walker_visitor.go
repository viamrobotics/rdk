package config

type Walker interface {
	// Walk walks a structure, possibly returning an error.
	Walk(Visitor) error
}

type Visitor interface {
	// Visit visits a node and returns a new node, with or without modifications.
	Visit(interface{}) (interface{}, error)
}
