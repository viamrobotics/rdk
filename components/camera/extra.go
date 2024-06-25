package camera

import "context"
// Extra is the type of value stored in the Contexts.
type (
	Extra map[string]interface{}
	key   int
)
// comment

var extraKey key

// NewContext returns a new Context that carries value Extra.
func NewContext(ctx context.Context, e Extra) context.Context {
	return context.WithValue(ctx, extraKey, e)
}

// FromContext returns the Extra value stored in ctx, if any.
func FromContext(ctx context.Context) (Extra, bool) {
	ext, ok := ctx.Value(extraKey).(Extra)
	return ext, ok
}
