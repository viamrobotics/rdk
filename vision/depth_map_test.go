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

	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if m.GetDepth(300, 300) != 749 {
		t.Errorf("wrong depth")
	}

	theMat := m.ToMat()

	m = NewDepthMapFromMat(theMat)
	if m.width != 1280 {
		t.Errorf("wrong width")
	}
	if m.height != 720 {
		t.Errorf("wrong height")
	}
	if m.GetDepth(300, 300) != 749 {
		t.Errorf("wrong depth")
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
	if m.GetDepth(300, 300) != 749 {
		t.Errorf("wrong depth")
	}

}
