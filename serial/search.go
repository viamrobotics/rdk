//go:build !linux && !darwin
// +build !linux,!darwin

package serial

// Search returns nothing here for unsupported platforms.
func Search(filter SearchFilter) []Description {
	return nil
}
