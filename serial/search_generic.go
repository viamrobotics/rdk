package serial

// SearchFilter specifies how to find a specific device. Right
// now it only supports filtering by type.
type SearchFilter struct {
	Type Type
}
