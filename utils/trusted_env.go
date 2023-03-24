package utils

import (
	"context"
	"sync/atomic"

	"github.com/pkg/errors"
)

type ctxKey byte

const ctxKeyTrusted ctxKey = iota

// IsTrustedEnvironment is used to check the trusted state of the runtime.
// Note: by default, if no context is set up, trust is assumed; be careful.
func IsTrustedEnvironment(ctx context.Context) bool {
	val, has := ctx.Value(ctxKeyTrusted).(*atomic.Bool)
	return !has || val != nil && val.Load()
}

// WithTrustedEnvironment is used to inform environment trust across boundaries.
func WithTrustedEnvironment(ctx context.Context, trusted bool) (context.Context, error) {
	alreadySet := ctx.Value(ctxKeyTrusted)
	if alreadySet != nil {
		currentTrust, ok := alreadySet.(*atomic.Bool)
		if !ok || currentTrust == nil {
			return nil, errors.Errorf("trust value is not a bool but %v (%T)", alreadySet, alreadySet)
		}
		if !currentTrust.Load() && trusted {
			return nil, errors.New("cannot elevate trust")
		}
		currentTrust.Store(trusted)
		return ctx, nil
	}
	var newTrust atomic.Bool
	newTrust.Store(trusted)
	return context.WithValue(ctx, ctxKeyTrusted, &newTrust), nil
}
