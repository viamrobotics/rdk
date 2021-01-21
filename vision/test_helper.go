package vision

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/edaniels/golog"
	"gocv.io/x/gocv"
)

type MultipleImageTestDebugger struct {
	name   string
	glob   string
	inroot string
	out    string

	html        strings.Builder
	currentFile string
}

func (d *MultipleImageTestDebugger) GotDebugImage(mat gocv.Mat, name string) {
	outFile := filepath.Join(d.out, name+"-"+filepath.Base(d.currentFile))
	if !strings.HasSuffix(outFile, ".png") {
		outFile = outFile + ".png"
	}
	gocv.IMWrite(outFile, mat)
	d.addImageCell(outFile)
}

func (d *MultipleImageTestDebugger) addImageCell(f string) {
	d.html.WriteString(fmt.Sprintf("<td><img src='%s' width=300/></td>", f))
}

type MultipleImageTestDebuggerProcessor interface {
	Process(d *MultipleImageTestDebugger, fn string, img Image) error
}

func NewMultipleImageTestDebugger(prefix, glob string) MultipleImageTestDebugger {

	d := MultipleImageTestDebugger{}
	d.glob = glob
	d.inroot = filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", prefix)
	d.name = strings.Replace(prefix, "/", "-", 100)

	var err error
	d.out, err = filepath.Abs("out")
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(d.out, 0775); err != nil {
		panic(err)
	}

	return d
}

func (d *MultipleImageTestDebugger) Process(x MultipleImageTestDebuggerProcessor) error {
	files, err := filepath.Glob(filepath.Join(d.inroot, d.glob))
	if err != nil {
		return err
	}

	d.html.WriteString("<html><body><table>")

	for _, f := range files {
		d.currentFile = f
		golog.Global.Debug(f)
		img, err := NewImageFromFile(f)
		if err != nil {
			return err
		}

		d.html.WriteString("<tr>")
		d.GotDebugImage(img.MatUnsafe(), "raw")

		err = x.Process(d, f, img)
		if err != nil {
			return err
		}

		d.html.WriteString("</tr>")
	}

	d.html.WriteString("</table></body></html>")

	htmlOutFile := filepath.Join(d.out, d.name+".html")
	golog.Global.Debug(htmlOutFile)
	return ioutil.WriteFile(htmlOutFile, []byte(d.html.String()), 0640)
}
