package utils

import "context"

// Closer is closable type in a TryClose.
type Closer interface {
	Close(context.Context) error
}
