package utils

import (
	"context"
	"time"
)

// MergeContext merges the two given contexts together and returns a new "child"
// context parented by the first context that will be cancelled either by the
// returned cancel function or when either of the two initial contexts are canceled.
// Note: This implies that the values will only come from the first argument's context.
func MergeContext(ctx, otherCtx context.Context) (context.Context, func()) {
	mergedCtx, mergedCtxCancel := context.WithCancel(ctx)
	return mergeContexs(otherCtx, mergedCtx, mergedCtxCancel)
}

// MergeContextWithTimeout merges the two given contexts together and returns a new "child"
// context parented by the first context that will be cancelled either by the
// returned cancel function, when either of the two initial contexts are canceled,
// or when the given timeout elapses.
// Note: This implies that the values will only come from the first argument's context.
func MergeContextWithTimeout(ctx, otherCtx context.Context, timeout time.Duration) (context.Context, func()) {
	mergedCtx, mergedCtxCancel := context.WithTimeout(ctx, timeout)
	return mergeContexs(otherCtx, mergedCtx, mergedCtxCancel)
}

// MergeContextWithDeadline merges the two given contexts together and returns a new "child"
// context parented by the first context that will be cancelled either by the
// returned cancel function, when either of the two initial contexts are canceled,
// or when the given deadline lapses.
// Note: This implies that the values will only come from the first argument's context.
func MergeContextWithDeadline(ctx, otherCtx context.Context, deadline time.Time) (context.Context, func()) {
	mergedCtx, mergedCtxCancel := context.WithDeadline(ctx, deadline)
	return mergeContexs(otherCtx, mergedCtx, mergedCtxCancel)
}

func mergeContexs(
	otherCtx context.Context,
	mergedCtx context.Context,
	mergedCtxCancel func(),
) (context.Context, func()) {
	mergeCtxDone := make(chan struct{})
	PanicCapturingGo(func() {
		defer close(mergeCtxDone)
		select {
		case <-mergedCtx.Done():
		case <-otherCtx.Done():
			mergedCtxCancel()
		}
	})
	return mergedCtx, func() {
		defer func() { <-mergeCtxDone }()
		mergedCtxCancel()
	}
}
