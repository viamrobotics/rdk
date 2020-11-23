package main

import (
	"fmt"
	"image/color"
	"os"
	"sort"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/widget"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/vision"
	"github.com/echolabsinc/robotcore/vision/chess"
)

type ColorList []color.RGBA

func (cl ColorList) Len() int {
	return len(cl)
}

func (cl ColorList) Less(i, j int) bool {
	if cl[i].R != cl[j].R {
		return cl[i].R < cl[j].R
	}

	if cl[i].G != cl[j].G {
		return cl[i].G < cl[j].G
	}

	return cl[i].B < cl[j].B
}

func (cl ColorList) Swap(i, j int) {
	x := cl[i]
	cl[i] = cl[j]
	cl[j] = x
}

func getUniqueColors(imgMat gocv.Mat) []color.RGBA {
	img, err := vision.NewImage(imgMat)
	if err != nil {
		panic(err)
	}
	colors := ColorList{}

	for x := 0; x < img.Cols(); x++ {
		for y := 0; y < img.Rows(); y++ {
			data := img.ColorRowCol(y, x)

			found := false
			for _, other := range colors {
				if vision.ColorDistance(data, other) < 25 {
					found = true
					break
				}
			}

			if found {
				continue
			}

			colors = append(colors, data)
			if len(colors)%1000 == 0 {
				fmt.Printf("%d\n", len(colors))
			}
		}
	}

	sort.Sort(colors)

	return colors

}

func main() {
	img := gocv.IMRead(os.Args[1], gocv.IMReadUnchanged)

	uniqueColors := getUniqueColors(img)

	if true {
		good := ""
		bad := ""
		for _, c := range uniqueColors {
			d := chess.MyPinkDistance(c)
			x := fmt.Sprintf("<div style='background-color:rgba(%d, %d, %d, 1)'>vision.Color{color.RGBA{%d, %d, %d, 0}, \"myPink\", \"pink\"},</div>\n", c.R, c.G, c.B, c.R, c.G, c.B)
			if d < 40 {
				good += x
			} else {
				bad += x
			}
		}

		fmt.Println(good)
		fmt.Println("<hr>")
		fmt.Println(bad)

		return
	}

	fmt.Printf("total colors to do %d\n", len(uniqueColors))

	a := app.New()
	w := a.NewWindow("Hello")
	lbl := widget.NewLabel("Hello Fyne!")

	currentColor := 0
	circle := canvas.NewRectangle(uniqueColors[currentColor])
	circle.SetMinSize(fyne.Size{100, 100})
	pcs := []fyne.CanvasObject{lbl, circle}
	w.SetContent(widget.NewHBox(pcs...))

	w.Canvas().SetOnTypedKey(func(k *fyne.KeyEvent) {
		lbl.SetText(string(k.Name))

		fmt.Printf("%d, %d, %d, %s\n",
			uniqueColors[currentColor].R,
			uniqueColors[currentColor].G,
			uniqueColors[currentColor].B,
			string(k.Name))

		currentColor++
		if currentColor >= len(uniqueColors) {
			w.Close()
		}
		circle.FillColor = uniqueColors[currentColor]
		circle.Refresh()
	})

	w.ShowAndRun()

}
