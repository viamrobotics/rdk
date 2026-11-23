package modmanager

import (
	"testing"

	"go.viam.com/test"
)

func TestPosixACLRoundTrip(t *testing.T) {
	acl := aclFromMode(0o750)
	decoded, err := decodePosixACL(acl.encode())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, decoded, test.ShouldResemble, acl)

	_, err = decodePosixACL([]byte{2, 0, 0})
	test.That(t, err, test.ShouldNotBeNil)
	_, err = decodePosixACL([]byte{1, 0, 0, 0})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestPosixACLWithUserPerms(t *testing.T) {
	t.Run("adds a sorted user entry and a mask matching the group bits", func(t *testing.T) {
		acl, changed := aclFromMode(0o750).withUserPerms(1000, aclPermExecute)
		test.That(t, changed, test.ShouldBeTrue)
		test.That(t, acl, test.ShouldResemble, posixACL{
			{tag: aclTagUserObj, perm: 7, id: aclUndefinedID},
			{tag: aclTagUser, perm: aclPermExecute, id: 1000},
			{tag: aclTagGroupObj, perm: 5, id: aclUndefinedID},
			{tag: aclTagMask, perm: 5, id: aclUndefinedID},
			{tag: aclTagOther, perm: 0, id: aclUndefinedID},
		})
	})

	t.Run("raising the mask does not widen the group's effective access", func(t *testing.T) {
		acl, changed := aclFromMode(0o700).withUserPerms(1000, aclPermRead|aclPermExecute)
		test.That(t, changed, test.ShouldBeTrue)
		for _, e := range acl {
			switch e.tag {
			case aclTagGroupObj:
				test.That(t, e.perm, test.ShouldEqual, 0)
			case aclTagMask:
				test.That(t, e.perm, test.ShouldEqual, aclPermRead|aclPermExecute)
			}
		}
	})

	t.Run("clamps entries the old mask was hiding", func(t *testing.T) {
		start := posixACL{
			{tag: aclTagUserObj, perm: 7, id: aclUndefinedID},
			{tag: aclTagUser, perm: 7, id: 2000},
			{tag: aclTagGroupObj, perm: 7, id: aclUndefinedID},
			{tag: aclTagMask, perm: 1, id: aclUndefinedID},
			{tag: aclTagOther, perm: 0, id: aclUndefinedID},
		}
		acl, _ := start.withUserPerms(1000, aclPermRead)
		test.That(t, acl, test.ShouldResemble, posixACL{
			{tag: aclTagUserObj, perm: 7, id: aclUndefinedID},
			{tag: aclTagUser, perm: aclPermRead, id: 1000},
			{tag: aclTagUser, perm: 1, id: 2000},
			{tag: aclTagGroupObj, perm: 1, id: aclUndefinedID},
			{tag: aclTagMask, perm: 5, id: aclUndefinedID},
			{tag: aclTagOther, perm: 0, id: aclUndefinedID},
		})
	})

	t.Run("no change when the user already has the perms", func(t *testing.T) {
		acl, _ := aclFromMode(0o750).withUserPerms(1000, aclPermRead|aclPermExecute)
		_, changed := acl.withUserPerms(1000, aclPermExecute)
		test.That(t, changed, test.ShouldBeFalse)
	})
}
