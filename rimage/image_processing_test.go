package rimage

import (
	"fmt"
	"image"
	"os"
	"testing"
)

func TestCanny1(t *testing.T) {
	doCannyTest(t, "canny1")
}

func TestCanny2(t *testing.T) {
	doCannyTest(t, "canny2")
}

func doCannyTest(t *testing.T, root string) {
	img, err := NewImageFromFile(fmt.Sprintf("data/%s.png", root))
	if err != nil {
		t.Fatal(err)
	}

	goodOnes := 0

	for a := 0.01; a <= .05; a += .005 {

		outfn := fmt.Sprintf("out/%s-%v.png", root, int(1000*a))
		fmt.Println(outfn)

		out, err := SimpleEdgeDetection(img, a, 3.0)
		if err != nil {
			t.Fatal(err)
		}

		os.MkdirAll("out", 0775)
		err = WriteImageToFile(outfn, out)
		if err != nil {
			t.Fatal(err)
		}

		bad := false

		for x := 50; x <= 750; x += 100 {
			for y := 50; y <= 750; y += 100 {

				spots := CountBrightSpots(out, image.Point{x, y}, 25, 255)

				if y < 200 || y > 600 {
					if spots < 100 {
						bad = true
						fmt.Printf("\t%v,%v %v\n", x, y, spots)
					}
				} else {
					if spots > 90 {
						bad = true
						fmt.Printf("\t%v,%v %v\n", x, y, spots)
					}
				}

				if bad {
					break
				}
			}

			if bad {
				break
			}
		}

		if bad {
			continue
		}

		goodOnes++

		break
	}

	if goodOnes == 0 {
		t.Errorf("no good ones found for root %s", root)
	}
}

func BenchmarkConvertImage(b *testing.B) {
	img, err := ReadImageFromFile("data/canny1.png")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		ConvertImage(img)
	}
}
