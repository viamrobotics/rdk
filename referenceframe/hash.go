package referenceframe

// Hash returns a hash value for this frame system.
func (sfs *FrameSystem) Hash() int {
	hash := len(sfs.frames) * 1000

	for k, f := range sfs.frames {
		// + is important
		hash += hashString(k)
		hash += f.Hash()
	}

	return hash
}

func hashString(s string) int {
	hash := 0
	for idx, c := range s {
		hash += ((idx + 1) * 7) + ((int(c) + 12) * 12)
	}
	return hash
}
