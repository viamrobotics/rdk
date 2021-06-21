package mongoutils_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"

	mongoutils "go.viam.com/core/utils/mongo"
)

func TestRegisterNamespace(t *testing.T) {
	db1, coll1 := primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()
	test.That(t, mongoutils.RegisterNamespace(&db1, &coll1), test.ShouldBeNil)
	ns := mongoutils.Namespaces()
	unNS := mongoutils.UnmanagedNamespaces()
	test.That(t, ns[db1], test.ShouldContain, coll1)
	test.That(t, unNS[db1], test.ShouldNotContain, coll1)
	db2 := primitive.NewObjectID().Hex()
	coll2 := coll1
	test.That(t, mongoutils.RegisterNamespace(&db2, &coll2), test.ShouldBeNil)
	ns = mongoutils.Namespaces()
	test.That(t, ns[db1], test.ShouldContain, coll1)

	err := mongoutils.RegisterNamespace(&db1, &coll2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, coll1)
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one location")
}

func TestRegisterUnmanagedNamespace(t *testing.T) {
	db1, coll1 := primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()
	test.That(t, mongoutils.RegisterUnmanagedNamespace(db1, coll1), test.ShouldBeNil)
	ns := mongoutils.Namespaces()
	unNS := mongoutils.UnmanagedNamespaces()
	test.That(t, ns[db1], test.ShouldNotContain, coll1)
	test.That(t, unNS[db1], test.ShouldContain, coll1)
	db2 := primitive.NewObjectID().Hex()
	coll2 := coll1
	test.That(t, mongoutils.RegisterUnmanagedNamespace(db2, coll2), test.ShouldBeNil)
	unNS = mongoutils.UnmanagedNamespaces()
	test.That(t, unNS[db1], test.ShouldContain, coll1)

	err := mongoutils.RegisterUnmanagedNamespace(db1, coll2)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, coll1)
	test.That(t, err.Error(), test.ShouldContainSubstring, "more than one location")
}

func TestRandomizeNamespaces(t *testing.T) {
	db1, coll1 := primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()
	test.That(t, mongoutils.RegisterNamespace(&db1, &coll1), test.ShouldBeNil)
	db2, coll2 := primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()
	test.That(t, mongoutils.RegisterUnmanagedNamespace(db2, coll2), test.ShouldBeNil)

	db1Copy, coll1Copy := db1, coll1

	ns := mongoutils.Namespaces()
	unNS := mongoutils.UnmanagedNamespaces()
	test.That(t, ns[db1], test.ShouldContain, coll1)
	test.That(t, unNS[db2], test.ShouldContain, coll2)

	newNS, restore := mongoutils.RandomizeNamespaces()
	test.That(t, newNS[db1], test.ShouldContain, coll1)
	test.That(t, newNS[db1Copy], test.ShouldNotContain, coll1Copy)

	ns = mongoutils.Namespaces()
	unNS = mongoutils.UnmanagedNamespaces()
	test.That(t, ns[db1], test.ShouldContain, coll1)
	test.That(t, ns[db1Copy], test.ShouldNotContain, coll1Copy)
	test.That(t, unNS[db2], test.ShouldContain, coll2)

	restore()

	ns = mongoutils.Namespaces()
	unNS = mongoutils.UnmanagedNamespaces()
	test.That(t, ns[db1], test.ShouldContain, coll1)
	test.That(t, unNS[db2], test.ShouldContain, coll2)
}
