package rimage

import (
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
)

type MultipleImageTestDebugger struct {
	T      *testing.T
	name   string
	glob   string
	inroot string
	out    string

	html        strings.Builder
	currentFile string

	pendingImages int32
}

func (d *MultipleImageTestDebugger) currentImgConfigFile() string {
	idx := strings.LastIndexByte(d.currentFile, '.')
	return fmt.Sprintf("%s.json", d.currentFile[0:idx])
}

func (d *MultipleImageTestDebugger) CurrentImgConfig(out interface{}) error {
	fn := d.currentImgConfigFile()

	file, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", fn, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(out)
}

func (d *MultipleImageTestDebugger) GotDebugImage(img image.Image, name string) {
	outFile := filepath.Join(d.out, name+"-"+filepath.Base(d.currentFile))
	if !strings.HasSuffix(outFile, ".png") {
		outFile = outFile + ".png"
	}
	atomic.AddInt32(&d.pendingImages, 1)
	go func() {
		err := WriteImageToFile(outFile, img)
		atomic.AddInt32(&d.pendingImages, -1)
		if err != nil {
			panic(err)
		}
	}()
	d.addImageCell(outFile)
}

func (d *MultipleImageTestDebugger) addImageCell(f string) {
	d.html.WriteString(fmt.Sprintf("<td><img src='%s' width=300/></td>", f))
}

type MultipleImageTestDebuggerProcessor interface {
	Process(d *MultipleImageTestDebugger, fn string, img image.Image) error
}

func NewMultipleImageTestDebugger(t *testing.T, prefix, glob string) MultipleImageTestDebugger {

	d := MultipleImageTestDebugger{}
	d.T = t
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

	defer func() {
		for {
			pending := atomic.LoadInt32(&d.pendingImages)
			if pending <= 0 {
				break
			}

			golog.Global.Debugf("sleeping for pending images %d", pending)

			time.Sleep(time.Duration(50*pending) * time.Millisecond)
		}
	}()

	numFiles := 0

	for _, f := range files {
		if !IsImageFile(f) {
			continue
		}

		numFiles++

		d.currentFile = f
		golog.Global.Debug(f)

		cont := d.T.Run(f, func(t *testing.T) {
			img, err := ReadImageFromFile(f)
			if err != nil {
				t.Fatal(err)
			}

			d.html.WriteString(fmt.Sprintf("<tr><td colspan=100>%s</td></tr>", f))
			d.html.WriteString("<tr>")
			d.GotDebugImage(img, "raw")

			err = x.Process(d, f, img)
			if err != nil {
				t.Fatalf("error processing file %s : %s", f, err)
			}

			d.html.WriteString("</tr>")
		})

		if !cont {
			return nil
		}
	}

	if numFiles == 0 {
		d.T.Skip("no input files")
		return nil
	}

	d.html.WriteString("</table></body></html>")

	htmlOutFile := filepath.Join(d.out, d.name+".html")
	golog.Global.Debug(htmlOutFile)

	return ioutil.WriteFile(htmlOutFile, []byte(d.html.String()), 0640)
}
