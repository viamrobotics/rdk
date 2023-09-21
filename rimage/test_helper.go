//go:build !no_cgo

package rimage

import (
	// for test_helper.html.
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/pointcloud"
)

//go:embed test_helper.html
var testHelperHTML string

type namedString struct {
	File string
	Name string
}

type oneTestOutput struct {
	Images []namedString
	PCDs   []namedString
}

func (one *oneTestOutput) addImageCell(outputFile, name string) {
	one.Images = append(one.Images, namedString{filepath.Base(outputFile), name})
}

func (one *oneTestOutput) addPCDCell(outputFile, name string) {
	one.PCDs = append(one.PCDs, namedString{filepath.Base(outputFile), name})
}

type testOutput struct {
	mu    sync.Mutex
	Files map[string]*oneTestOutput
}

func (to *testOutput) getFile(testFile string) *oneTestOutput {
	to.mu.Lock()
	defer to.mu.Unlock()

	if to.Files == nil {
		to.Files = map[string]*oneTestOutput{}
	}

	one := to.Files[testFile]
	if one == nil {
		one = &oneTestOutput{}
		to.Files[testFile] = one
	}

	return one
}

// MultipleImageTestDebugger TODO.
type MultipleImageTestDebugger struct {
	name            string
	glob            string
	inrootPrimary   string
	inrootSecondary string
	out             string

	output testOutput

	pendingImages int32
	logger        golog.Logger
}

// ProcessorContext TODO.
type ProcessorContext struct {
	d           *MultipleImageTestDebugger
	currentFile string
	output      *testOutput
}

func (pCtx *ProcessorContext) currentImgConfigFile() string {
	idx := strings.LastIndexByte(pCtx.currentFile, '.')
	return fmt.Sprintf("%s.json", pCtx.currentFile[0:idx])
}

// CurrentImgConfig TODO.
func (pCtx *ProcessorContext) CurrentImgConfig(out interface{}) error {
	fn := pCtx.currentImgConfigFile()

	//nolint:gosec
	file, err := os.Open(fn)
	if err != nil {
		return errors.Wrapf(err, "error opening %s", fn)
	}
	defer utils.UncheckedErrorFunc(file.Close)

	decoder := json.NewDecoder(file)
	return decoder.Decode(out)
}

// GotDebugImage TODO.
func (pCtx *ProcessorContext) GotDebugImage(img image.Image, name string) {
	outFile := filepath.Join(pCtx.d.out, name+"-"+filepath.Base(pCtx.currentFile))
	if !strings.HasSuffix(outFile, ".png") {
		outFile += ".png"
	}
	atomic.AddInt32(&pCtx.d.pendingImages, 1)
	utils.PanicCapturingGo(func() {
		err := WriteImageToFile(outFile, img)
		atomic.AddInt32(&pCtx.d.pendingImages, -1)
		if err != nil {
			panic(err)
		}
	})
	pCtx.output.getFile(pCtx.currentFile).addImageCell(outFile, name)
}

// GotDebugPointCloud TODO
// in order to use this, you'll have to run a webserver from the output directory of the html
// something like: python3 -m http.server will work.
func (pCtx *ProcessorContext) GotDebugPointCloud(pc pointcloud.PointCloud, name string) {
	outFile := filepath.Join(pCtx.d.out, name+"-"+filepath.Base(pCtx.currentFile))
	if !strings.HasSuffix(outFile, ".pcd") {
		outFile += ".pcd"
	}
	atomic.AddInt32(&pCtx.d.pendingImages, 1)
	go func() {
		//nolint:gosec
		f, err := os.OpenFile(outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			panic(err)
		}
		err = pointcloud.ToPCD(pc, f, pointcloud.PCDBinary)
		if err != nil {
			panic(err)
		}

		atomic.AddInt32(&pCtx.d.pendingImages, -1)
	}()
	pCtx.output.getFile(pCtx.currentFile).addPCDCell(outFile, name)
}

// MultipleImageTestDebuggerProcessor TODO.
type MultipleImageTestDebuggerProcessor interface {
	Process(
		t *testing.T,
		pCtx *ProcessorContext,
		fn string,
		img image.Image,
		img2 image.Image,
		logger golog.Logger,
	) error
}

const debugTestEnvVar = "VIAM_DEBUG"

func checkSkipDebugTest(t *testing.T) {
	t.Helper()
	if os.Getenv(debugTestEnvVar) == "" {
		t.Skipf("set environment variable %q to run this test", debugTestEnvVar)
	}
}

// NewMultipleImageTestDebugger TODO.
func NewMultipleImageTestDebugger(t *testing.T, prefixOne, glob, prefixTwo string) *MultipleImageTestDebugger {
	checkSkipDebugTest(t)
	t.Helper()
	d := MultipleImageTestDebugger{logger: golog.NewTestLogger(t)}
	d.glob = glob
	d.inrootPrimary = artifact.MustPath(prefixOne)
	if prefixTwo != "" {
		d.inrootSecondary = artifact.MustPath(prefixTwo)
	}
	d.name = prefixOne + "-" + t.Name()
	d.name = strings.Replace(d.name, "/", "-", 100)
	d.name = strings.Replace(d.name, " ", "-", 100)
	d.out = t.TempDir()
	return &d
}

// Process TODO.
func (d *MultipleImageTestDebugger) Process(t *testing.T, x MultipleImageTestDebuggerProcessor) (err error) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(d.inrootPrimary, d.glob))
	if err != nil {
		return err
	}

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
		t.Helper()
		for _, f := range files {
			if !IsImageFile(f) {
				continue
			}

			numFiles++

			currentFile := f
			fileName := filepath.Base(f)
			// check if there is a secondary file of the same name
			fileSecondary := filepath.Join(d.inrootSecondary, fileName)
			t.Run(filepath.Base(f), func(t *testing.T) {
				t.Helper()
				t.Parallel()
				d.logger.Debug(currentFile)
				img, err := readImageFromFile(currentFile)
				test.That(t, err, test.ShouldBeNil)
				var img2 image.Image
				if _, err := os.Stat(fileSecondary); err == nil {
					img2, err = readImageFromFile(fileSecondary)
					test.That(t, err, test.ShouldBeNil)
				}

				pCtx := &ProcessorContext{
					d:           d,
					currentFile: currentFile,
					output:      &d.output,
				}

				pCtx.GotDebugImage(img, "raw")
				if img2 != nil {
					pCtx.GotDebugImage(img2, "raw_second")
				}

				logger := golog.NewTestLogger(t)
				err = x.Process(t, pCtx, currentFile, img, img2, logger)
				test.That(t, err, test.ShouldBeNil)
			})
		}
	})

	if numFiles == 0 {
		t.Skip("no input files")
		return nil
	}

	theTemplate := template.New("foo")
	_, err = theTemplate.Parse(testHelperHTML)
	if err != nil {
		return err
	}

	htmlOutFile := filepath.Join(d.out, d.name+".html")
	d.logger.Debug(htmlOutFile)

	//nolint:gosec
	outFile, err := os.OpenFile(htmlOutFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, outFile.Close())
	}()
	return theTemplate.Execute(outFile, &d.output)
}
