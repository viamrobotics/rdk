package modmanager

import (
	"encoding/binary"
	"sort"

	"github.com/pkg/errors"
)

// Linux POSIX ACL xattr encoding (see linux/posix_acl_xattr.h).
const (
	aclXattrVersion = 2
	aclTagUserObj   = 0x01
	aclTagUser      = 0x02
	aclTagGroupObj  = 0x04
	aclTagGroup     = 0x08
	aclTagMask      = 0x10
	aclTagOther     = 0x20
	aclUndefinedID  = 0xFFFFFFFF
	aclEntrySize    = 8
	aclHeaderSize   = 4

	aclPermExecute = 1
	aclPermRead    = 4
)

type aclEntry struct {
	tag  uint16
	perm uint16
	id   uint32
}

type posixACL []aclEntry

func aclFromMode(mode uint32) posixACL {
	return posixACL{
		{tag: aclTagUserObj, perm: uint16(mode>>6) & 7, id: aclUndefinedID},
		{tag: aclTagGroupObj, perm: uint16(mode>>3) & 7, id: aclUndefinedID},
		{tag: aclTagOther, perm: uint16(mode) & 7, id: aclUndefinedID},
	}
}

func decodePosixACL(data []byte) (posixACL, error) {
	if len(data) < aclHeaderSize || (len(data)-aclHeaderSize)%aclEntrySize != 0 {
		return nil, errors.Errorf("malformed POSIX ACL of %d bytes", len(data))
	}
	if version := binary.LittleEndian.Uint32(data); version != aclXattrVersion {
		return nil, errors.Errorf("unsupported POSIX ACL version %d", version)
	}
	var acl posixACL
	for off := aclHeaderSize; off < len(data); off += aclEntrySize {
		acl = append(acl, aclEntry{
			tag:  binary.LittleEndian.Uint16(data[off:]),
			perm: binary.LittleEndian.Uint16(data[off+2:]),
			id:   binary.LittleEndian.Uint32(data[off+4:]),
		})
	}
	return acl, nil
}

func (acl posixACL) encode() []byte {
	data := make([]byte, aclHeaderSize+aclEntrySize*len(acl))
	binary.LittleEndian.PutUint32(data, aclXattrVersion)
	for i, e := range acl {
		off := aclHeaderSize + i*aclEntrySize
		binary.LittleEndian.PutUint16(data[off:], e.tag)
		binary.LittleEndian.PutUint16(data[off+2:], e.perm)
		binary.LittleEndian.PutUint32(data[off+4:], e.id)
	}
	return data
}

// withUserPerms returns acl with uid granted at least perm, and whether that changed anything. Raising the mask
// would also raise other masked entries' effective access, so those are first clamped to what they had.
func (acl posixACL) withUserPerms(uid uint32, perm uint16) (posixACL, bool) {
	out := make(posixACL, len(acl))
	copy(out, acl)

	userIdx, maskIdx := -1, -1
	for i, e := range out {
		switch {
		case e.tag == aclTagUser && e.id == uid:
			userIdx = i
		case e.tag == aclTagMask:
			maskIdx = i
		}
	}

	if maskIdx == -1 {
		var groupPerm uint16
		for _, e := range out {
			if e.tag == aclTagGroupObj {
				groupPerm = e.perm
			}
		}
		out = append(out, aclEntry{tag: aclTagMask, perm: groupPerm, id: aclUndefinedID})
		maskIdx = len(out) - 1
	}

	changed := false
	oldMask := out[maskIdx].perm
	if oldMask&perm != perm {
		for i, e := range out {
			if e.tag == aclTagUser || e.tag == aclTagGroup || e.tag == aclTagGroupObj {
				out[i].perm = e.perm & oldMask
			}
		}
		out[maskIdx].perm = oldMask | perm
		changed = true
	}

	if userIdx == -1 {
		out = append(out, aclEntry{tag: aclTagUser, perm: perm, id: uid})
		changed = true
	} else if out[userIdx].perm&perm != perm {
		out[userIdx].perm |= perm
		changed = true
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].tag != out[j].tag {
			return out[i].tag < out[j].tag
		}
		return out[i].id < out[j].id
	})
	return out, changed
}
