package config

type Walker interface {
	// Walk walks a structure and returns a new structure, with or without modifications.
	Walk(Visitor) (interface{}, error)
}

type Visitor interface {
	// Visit visits a node and returns a new node, with or without modifications.
	Visit(interface{}) (interface{}, error)
}
