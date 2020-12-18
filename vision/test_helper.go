package vision

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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
	gocv.IMWrite(outFile, mat)
	d.addImageCell(outFile)
}

func (d *MultipleImageTestDebugger) addImageCell(f string) {
	d.html.WriteString(fmt.Sprintf("<td><img src='%s' width=300/></td>", f))
}

type MultipleImageTestDebuggerProcessor interface {
	Process(d *MultipleImageTestDebugger, fn string, img gocv.Mat) error
}

func NewMultipleImageTestDebugger(prefix, glob string) MultipleImageTestDebugger {
	var err error

	d := MultipleImageTestDebugger{}
	d.glob = glob
	d.inroot = filepath.Join(os.Getenv("HOME"), "/Dropbox/echolabs_data/", prefix)
	d.out, err = filepath.Abs("out")
	d.name = strings.Replace(prefix, "/", "-", 100)

	if err != nil {
		panic(err)
	}

	os.MkdirAll(d.out, 0775)

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
		fmt.Println(f)
		img := gocv.IMRead(f, gocv.IMReadUnchanged)

		d.html.WriteString("<tr>")
		d.addImageCell(f)

		err := x.Process(d, f, img)
		if err != nil {
			return err
		}

		d.html.WriteString("</tr>")
	}

	d.html.WriteString("</table></body></html>")

	htmlOutFile := filepath.Join(d.out, d.name+".html")
	fmt.Println(htmlOutFile)
	return ioutil.WriteFile(htmlOutFile, []byte(d.html.String()), 0640)
}
