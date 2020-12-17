package vision

import (
	"bytes"
	"testing"
)

func TestDepthMap1(t *testing.T) {
	m, err := ParseDepthMap("chess/data/board2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}
	m.Smooth()

	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}

	origHeight := m.GetDepth(300, 300)
	if origHeight != 749 {
		t.Errorf("wrong depth %v", m.GetDepth(300, 300))
	}
	origHeight2 := m.GetDepth(17, 111)

	theMat := m.ToMat()

	m = NewDepthMapFromMat(theMat)

	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if m.GetDepth(300, 300) != origHeight {
		t.Errorf("wrong depth %v != %v", m.GetDepth(300, 300), origHeight)
	}
	if m.GetDepth(17, 111) != origHeight2 {
		t.Errorf("wrong depth %v != %v", m.GetDepth(17, 111), origHeight2)
	}

	buf := bytes.Buffer{}
	err = m.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}

	m, err = ReadDepthMap(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if origHeight != 749 {
		t.Errorf("wrong depth")
	}

}

func TestDepthMapNewFormat(t *testing.T) {
	m, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		t.Fatal(err)
	}

	if m.width != 1280 || m.height != 720 {
		t.Errorf("width and height wrong %v %v", m.width, m.height)
	}

	numZero := 0

	for x := 0; x < m.width; x++ {
		d := m.GetDepth(x, m.height-1)
		if d == 0 {
			numZero = numZero + 1
		} else {
			if d < 100 || d > 5000 {
				t.Errorf("weird depth %v", d)
			}
		}

	}

	if numZero == 0 || numZero >= m.width {
		t.Errorf("numZero wrong %v", numZero)
	}
}
