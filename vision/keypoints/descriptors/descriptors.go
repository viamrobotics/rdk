// descriptors holds the definitions for our BRIEF descriptors.
package descriptors

// Descriptor stores a descriptor in a slice of uint64.
type Descriptor struct {
	Bits []uint64
}

// Descriptors stores a slice of Descriptor.
type Descriptors struct {
	Descriptors []Descriptor
}
