package vision

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"gocv.io/x/gocv"
)

func TestChess1(t *testing.T) {
	os.MkdirAll("out/data/", 0775)
	files, err := filepath.Glob("data/*.jpg")
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		img := gocv.IMRead(f, gocv.IMReadUnchanged)
		process(img)
		//process2(img)
		gocv.IMWrite("out/"+f, img)

	}

}

func TestChess2(t *testing.T) {

	root := filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/chess/upclose1/")

	os.MkdirAll(filepath.Join(root, "out"), 0775)

	files, err := filepath.Glob(filepath.Join(root, "*.jpg"))
	if err != nil {
		t.Fatal(err)
	}

	html := "<html><body><table>"

	for _, f := range files {
		img := gocv.IMRead(f, gocv.IMReadUnchanged)
		process(img)
		//process2(img)

		out := filepath.Join(root, "out", filepath.Base(f))
		gocv.IMWrite(out, img)

		html = fmt.Sprintf("%s<tr><td><img src='%s' width=300 /></td><td><img src='%s' width=300 /></td></tr>\n", html, f, out)
	}

	html = html + "</table></body></html>"
	err = ioutil.WriteFile("upclose1-all.html", []byte(html), 0640)
	if err != nil {
		t.Fatal(err)
	}

}
