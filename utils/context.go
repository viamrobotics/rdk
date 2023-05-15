package utils

import (
	"context"
)

// TODO: add documentation to this file
type ContextWithMetadata struct {
	context.Context
	md map[string]string
}

func NewContextWithMetadata(ctx context.Context) context.Context {
	return &ContextWithMetadata{
		Context: ctx,
		md:      make(map[string]string),
	}
}

func (ctx *ContextWithMetadata) Value(key any) any {
	if s, ok := key.(string); ok {
		if v, ok := ctx.md[s]; ok {
			return v
		}
	}

	return ctx.Context.Value(key)
}

func (ctx *ContextWithMetadata) WithValue(key, value string) {
	ctx.md[key] = value
	return
}
