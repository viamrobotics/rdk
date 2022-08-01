package utils

import "context"

// Closer is closable in a TryClose
type Closer interface {
	Close(context.Context) error
}
