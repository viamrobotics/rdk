package chess

import (
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gocv.io/x/gocv"
)

type FileTestStuff struct {
	prefix string
	glob   string
	root   string
	out    string
}

type P func(gocv.Mat, *gocv.Mat) ([]image.Point, error)

func NewFileTestStuff(prefix, glob string) FileTestStuff {
	fts := FileTestStuff{}
	fts.prefix = prefix
	fts.glob = glob
	fts.root = filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", fts.prefix)
	fts.out = filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", fts.prefix, "out")

	os.MkdirAll(fts.out, 0775)

	return fts
}

func (fts *FileTestStuff) Process(outputfile string, x P) {
	files, err := filepath.Glob(filepath.Join(fts.root, fts.glob))
	if err != nil {
		panic(err)
	}

	html := "<html><body><table>"

	for _, f := range files {
		fmt.Println(f)
		img := gocv.IMRead(f, gocv.IMReadUnchanged)

		out := gocv.NewMat()
		defer out.Close()
		corners, err := x(img, &out)
		if err != nil {
			panic(err)
		}

		outFile := filepath.Join(fts.out, filepath.Base(f))
		warpedOutFile := filepath.Join(fts.out, "warped-"+filepath.Base(f))

		gocv.IMWrite(outFile, out)

		if corners != nil {
			warped, _, err := warpColorAndDepthToChess(img, gocv.Mat{}, corners)
			if err != nil {
				panic(err)
			}

			gocv.IMWrite(warpedOutFile, warped)

		}

		html = fmt.Sprintf("%s<tr><td><img src='%s' width=300 /></td><td><img src='%s' width=300 /></td><td><img src='%s' width=300 height=225 /></td></tr>\n", html, f, outFile, warpedOutFile)
	}

	html = html + "</table></body></html>"
	err = ioutil.WriteFile(outputfile, []byte(html), 0640)
	if err != nil {
		panic(err)
	}

}

/*
func TestChess1(t *testing.T) {
	os.MkdirAll("out", 0775)
	fts := NewFileTestStuff("chess/boardseliot1", "*.png")
	fts.Process("out/boardseliot1.html", processFindCornersBad)
}
*/

func TestChessCheatRed1(t *testing.T) {
	os.MkdirAll("out", 0775)
	fts := NewFileTestStuff("chess/boardseliot2", "*.png")
	fts.Process("out/boardseliot2.html", FindChessCornersPinkCheat)
}
