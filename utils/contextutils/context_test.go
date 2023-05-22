package contextutils

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestContextWithMetadata(t *testing.T) {
	// nothing in context
	ctx := context.Background()
	mdFromContext := ctx.Value(MetadataKey)
	test.That(t, mdFromContext, test.ShouldBeNil)

	// initialize metadata, empty at first
	ctx, md := ContextWithMetadata(ctx)
	test.That(t, md, test.ShouldBeEmpty)
	test.That(t, ctx.Value(MetadataKey), test.ShouldBeEmpty)

	// add values to local metadata and show context metadata has updated
	k, v := "hello", []string{"world"}
	md[k] = v
	mdFromContext = ctx.Value(MetadataKey)
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
	ctx = context.WithValue(ctx, MetadataKey, someString)
	mdFromContext = ctx.Value(MetadataKey)
	mdString, ok := mdFromContext.(string)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, mdString, test.ShouldEqual, someString)

	ctx, md = ContextWithMetadata(ctx)
	test.That(t, md, test.ShouldBeEmpty)

	mdFromContext = ctx.Value(MetadataKey)
	mdMap, ok = mdFromContext.(map[string][]string)
	test.That(t, ok, test.ShouldEqual, true)
	test.That(t, mdMap, test.ShouldBeEmpty)
	test.That(t, ctx.Value(MetadataKey), test.ShouldBeEmpty)
}
