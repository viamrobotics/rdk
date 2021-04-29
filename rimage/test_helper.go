package rimage

import (
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/robotcore/artifact"
)

type MultipleImageTestDebugger struct {
	mu            sync.Mutex
	name          string
	glob          string
	inroot        string
	out           string
	imagesAligned bool

	html strings.Builder

	pendingImages int32
	logger        golog.Logger
}

type ProcessorContext struct {
	d           *MultipleImageTestDebugger
	currentFile string
	html        strings.Builder
}

func (pCtx *ProcessorContext) currentImgConfigFile() string {
	idx := strings.LastIndexByte(pCtx.currentFile, '.')
	return fmt.Sprintf("%s.json", pCtx.currentFile[0:idx])
}

func (pCtx *ProcessorContext) CurrentImgConfig(out interface{}) error {
	fn := pCtx.currentImgConfigFile()

	file, err := os.Open(fn)
	if err != nil {
		return fmt.Errorf("error opening %s: %w", fn, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(out)
}

func (pCtx *ProcessorContext) GotDebugImage(img image.Image, name string) {
	outFile := filepath.Join(pCtx.d.out, name+"-"+filepath.Base(pCtx.currentFile))
	if !strings.HasSuffix(outFile, ".png") {
		outFile = outFile + ".png"
	}
	atomic.AddInt32(&pCtx.d.pendingImages, 1)
	go func() {
		err := WriteImageToFile(outFile, img)
		atomic.AddInt32(&pCtx.d.pendingImages, -1)
		if err != nil {
			panic(err)
		}
	}()
	pCtx.addImageCell(outFile)
}

func (pCtx *ProcessorContext) addImageCell(f string) {
	pCtx.html.WriteString(fmt.Sprintf("<td><img src='%s' width=300/></td>", f))
}

type MultipleImageTestDebuggerProcessor interface {
	Process(
		t *testing.T,
		pCtx *ProcessorContext,
		fn string,
		img image.Image,
		logger golog.Logger,
	) error
}

func NewMultipleImageTestDebugger(t *testing.T, prefix, glob string, imagesAligned bool) *MultipleImageTestDebugger {
	d := MultipleImageTestDebugger{logger: golog.NewTestLogger(t)}
	d.imagesAligned = imagesAligned
	d.glob = glob
	d.inroot = artifact.MustPath(prefix)
	d.name = prefix + "-" + t.Name()
	d.name = strings.Replace(d.name, "/", "-", 100)
	d.name = strings.Replace(d.name, " ", "-", 100)

	var err error
	d.out, err = filepath.Abs("out")
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(d.out, 0775); err != nil {
		panic(err)
	}

	return &d
}

func (d *MultipleImageTestDebugger) Process(t *testing.T, x MultipleImageTestDebuggerProcessor) error {
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

			d.logger.Debugf("sleeping for pending images %d", pending)

			time.Sleep(time.Duration(50*pending) * time.Millisecond)
		}
	}()

	numFiles := 0

	// group and block parallel runs by having a subtest parent
	t.Run("files", func(t *testing.T) {
		for _, f := range files {
			if !IsImageFile(f) {
				continue
			}

			numFiles++

			currentFile := f
			d.logger.Debug(currentFile)

			t.Run(currentFile, func(t *testing.T) {
				t.Parallel()
				img, err := readImageFromFile(currentFile, d.imagesAligned)
				if err != nil {
					t.Fatalf("cannot read %s : %s", currentFile, err)
				}

				pCtx := &ProcessorContext{
					d:           d,
					currentFile: currentFile,
				}

				pCtx.html.WriteString(fmt.Sprintf("<tr><td colspan=100>%s</td></tr>", currentFile))
				pCtx.html.WriteString("<tr>")
				pCtx.GotDebugImage(img, "raw")

				logger := golog.NewTestLogger(t)
				err = x.Process(t, pCtx, currentFile, img, logger)
				if err != nil {
					t.Fatalf("error processing file %s : %s", currentFile, err)
				}

				pCtx.html.WriteString("</tr>")
				d.mu.Lock()
				d.html.WriteString(pCtx.html.String())
				d.mu.Unlock()
			})
		}
	})

	if numFiles == 0 {
		t.Skip("no input files")
		return nil
	}

	d.html.WriteString("</table></body></html>")

	htmlOutFile := filepath.Join(d.out, d.name+".html")
	d.logger.Debug(htmlOutFile)

	return ioutil.WriteFile(htmlOutFile, []byte(d.html.String()), 0640)
}
