package utils

import "context"

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
		return ctx.md[s]
	}

	return ""
}

func (ctx *ContextWithMetadata) WithValue(key, value string) {
	ctx.md[key] = value
	return
}
