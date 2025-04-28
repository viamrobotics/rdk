package sys

func Str32(chars [32]byte) string {
	var i = 0
	var c byte
	for i, c = range chars {
		if c == 0 {
			break
		}
	}
	return string(chars[:i])
}

func Char32(str string) (chars [32]byte) {
	copy(chars[:31], str)
	return
}
