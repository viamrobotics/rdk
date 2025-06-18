package h264

import (
	"fmt"
)

// AnnexBUnmarshal decodes an access unit from the Annex-B stream format.
// Specification: ITU-T Rec. H.264, Annex B
func AnnexBUnmarshal(buf []byte) ([][]byte, error) {
	bl := len(buf)
	initZeroCount := 0
	start := 0

outer:
	for {
		if start >= bl || start >= 4 {
			return nil, fmt.Errorf("initial delimiter not found")
		}

		switch initZeroCount {
		case 0, 1:
			if buf[start] != 0 {
				return nil, fmt.Errorf("initial delimiter not found")
			}
			initZeroCount++

		case 2, 3:
			switch buf[start] {
			case 1:
				start++
				break outer

			case 0:

			default:
				return nil, fmt.Errorf("initial delimiter not found")
			}
			initZeroCount++
		}

		start++
	}

	zeroCount := 0
	n := 0

	for i := start; i < bl; i++ {
		switch buf[i] {
		case 0:
			zeroCount++

		case 1:
			if zeroCount == 2 || zeroCount == 3 {
				n++
			}
			zeroCount = 0

		default:
			zeroCount = 0
		}
	}

	if (n + 1) > MaxNALUsPerAccessUnit {
		return nil, fmt.Errorf("NALU count (%d) exceeds maximum allowed (%d)",
			n+1, MaxNALUsPerAccessUnit)
	}

	ret := make([][]byte, n+1)
	pos := 0
	start = initZeroCount + 1
	zeroCount = 0
	delimStart := 0
	auSize := 0

	for i := start; i < bl; i++ {
		switch buf[i] {
		case 0:
			if zeroCount == 0 {
				delimStart = i
			}
			zeroCount++

		case 1:
			if zeroCount == 2 || zeroCount == 3 {
				l := delimStart - start

				if l == 0 {
					return nil, fmt.Errorf("invalid NALU")
				}

				if (auSize + l) > MaxAccessUnitSize {
					return nil, fmt.Errorf("access unit size (%d) is too big, maximum is %d", auSize+l, MaxAccessUnitSize)
				}

				ret[pos] = buf[start:delimStart]
				pos++
				auSize += l
				start = i + 1
			}
			zeroCount = 0

		default:
			zeroCount = 0
		}
	}

	l := bl - start

	if l == 0 {
		return nil, fmt.Errorf("invalid NALU")
	}

	if (auSize + l) > MaxAccessUnitSize {
		return nil, fmt.Errorf("access unit size (%d) is too big, maximum is %d", auSize+l, MaxAccessUnitSize)
	}

	ret[pos] = buf[start:bl]

	return ret, nil
}

func annexBMarshalSize(au [][]byte) int {
	n := 0
	for _, nalu := range au {
		n += 4 + len(nalu)
	}
	return n
}

// AnnexBMarshal encodes an access unit into the Annex-B stream format.
// Specification: ITU-T Rec. H.264, Annex B
func AnnexBMarshal(au [][]byte) ([]byte, error) {
	buf := make([]byte, annexBMarshalSize(au))
	pos := 0

	for _, nalu := range au {
		pos += copy(buf[pos:], []byte{0x00, 0x00, 0x00, 0x01})
		pos += copy(buf[pos:], nalu)
	}

	return buf, nil
}
