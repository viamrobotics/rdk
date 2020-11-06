package vision


import (
	"fmt"
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

type P func(gocv.Mat, *gocv.Mat)

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
		img := gocv.IMRead(f, gocv.IMReadUnchanged)

		out := gocv.NewMat()
		defer out.Close()
		x(img, &out)

		outFile := filepath.Join(fts.out, filepath.Base(f))
		gocv.IMWrite(outFile, out)

		html = fmt.Sprintf("%s<tr><td><img src='%s' width=300 /></td><td><img src='%s' width=300 /></td></tr>\n", html, f, outFile)
	}

	html = html + "</table></body></html>"
	err = ioutil.WriteFile(outputfile, []byte(html), 0640)
	if err != nil {
		panic(err)
	}

}

func TestChess1(t *testing.T) {

	fts := NewFileTestStuff("chess/boardseliot1", "*.png")
	os.MkdirAll("out", 0775)
	fts.Process("out/boardseliot1.html", process)
}

