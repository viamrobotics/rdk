package contextutils

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestContextWithMetadata(t *testing.T) {
	// nothing in context
	ctx := context.Background()
	mdFromContext := ctx.Value(MetadataContextKey)
	test.That(t, mdFromContext, test.ShouldBeNil)

	// initialize metadata, empty at first
	ctx, md := ContextWithMetadata(ctx)
	test.That(t, md, test.ShouldBeEmpty)
	test.That(t, ctx.Value(MetadataContextKey), test.ShouldBeEmpty)

	// add values to local metadata and show context metadata has updated
	k, v := "hello", []string{"world"}
	md[k] = v
	mdFromContext = ctx.Value(MetadataContextKey)
	mdMap, ok := mdFromContext.(map[string][]string)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, mdMap[k], test.ShouldResemble, v)

	// after calling ContextWithMetadata second time, old metadata still there
	ctx, md = ContextWithMetadata(ctx)
	test.That(t, md[k], test.ShouldResemble, v)

	// if metadata value gets overwritten with non-metadata value, next call to
	// ContextWithMetadata will add viam-metadata again, but will not be able to access old
	// metadata
	someString := "iamastring"
	ctx = context.WithValue(ctx, MetadataContextKey, someString)
	mdFromContext = ctx.Value(MetadataContextKey)
	mdString, ok := mdFromContext.(string)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, mdString, test.ShouldEqual, someString)

	ctx, md = ContextWithMetadata(ctx)
	test.That(t, md, test.ShouldBeEmpty)

	mdFromContext = ctx.Value(MetadataContextKey)
	mdMap, ok = mdFromContext.(map[string][]string)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, mdMap, test.ShouldBeEmpty)
	test.That(t, ctx.Value(MetadataContextKey), test.ShouldBeEmpty)
}

func TestContextWithTimeoutIfNoDeadline(t *testing.T) {
	// Test with no set deadline
	noDeadlineCtx := context.Background()
	noDeadlineCtxDeadline, _ := noDeadlineCtx.Deadline()
	deadlineCtx, cancel := ContextWithTimeoutIfNoDeadline(noDeadlineCtx, time.Second)
	defer cancel()
	deadlineCtxDeadline, _ := deadlineCtx.Deadline()
	test.That(t, deadlineCtxDeadline.After(noDeadlineCtxDeadline), test.ShouldBeTrue)

	// Test with prev set deadline
	ctx, cancel := ContextWithTimeoutIfNoDeadline(deadlineCtx, time.Second)
	defer cancel()
	ctxDeadline, _ := ctx.Deadline()
	test.That(t, ctxDeadline, test.ShouldEqual, deadlineCtxDeadline)
}
