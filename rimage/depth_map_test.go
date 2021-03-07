package rimage

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDepthMap1(t *testing.T) {
	m, err := ParseDepthMap("data/board2.dat.gz")
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

	buf := bytes.Buffer{}
	err = m.WriteTo(&buf)
	if err != nil {
		t.Fatal(err)
	}

	m, err = ReadDepthMap(bufio.NewReader(&buf))
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

func TestDepthRotate90(t *testing.T) {
	dm := NewEmptyDepthMap(2, 2)
	dm.Set(0, 0, 1)
	dm.Set(1, 0, 2)
	dm.Set(0, 1, 3)
	dm.Set(1, 1, 4)

	dm2 := dm.Rotate90(true)

	assert.Equal(t, Depth(1), dm2.GetDepth(0, 0))
}

func BenchmarkDepthMapRotate90(b *testing.B) {
	dm, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate90(true)
	}

}

func BenchmarkDepthMapRotate180(b *testing.B) {
	dm, err := ParseDepthMap("data/depthformat2.dat.gz")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		dm.Rotate180()
	}

}
