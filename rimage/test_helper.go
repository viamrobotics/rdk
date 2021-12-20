package rimage

import (
	_ "embed" // for test_helper.html
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

	"go.uber.org/multierr"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"

	"go.viam.com/core/pointcloud"
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

// MultipleImageTestDebugger TODO
type MultipleImageTestDebugger struct {
	name          string
	glob          string
	inroot        string
	out           string
	imagesAligned bool

	output testOutput

	pendingImages int32
	logger        golog.Logger
}

// ProcessorContext TODO
type ProcessorContext struct {
	d           *MultipleImageTestDebugger
	currentFile string
	output      *testOutput
}

func (pCtx *ProcessorContext) currentImgConfigFile() string {
	idx := strings.LastIndexByte(pCtx.currentFile, '.')
	return fmt.Sprintf("%s.json", pCtx.currentFile[0:idx])
}

// CurrentImgConfig TODO
func (pCtx *ProcessorContext) CurrentImgConfig(out interface{}) error {
	fn := pCtx.currentImgConfigFile()

	file, err := os.Open(fn)
	if err != nil {
		return errors.Wrapf(err, "error opening %s", fn)
	}
	defer utils.UncheckedErrorFunc(file.Close)

	decoder := json.NewDecoder(file)
	return decoder.Decode(out)
}

// GotDebugImage TODO
func (pCtx *ProcessorContext) GotDebugImage(img image.Image, name string) {
	outFile := filepath.Join(pCtx.d.out, name+"-"+filepath.Base(pCtx.currentFile))
	if !strings.HasSuffix(outFile, ".png") {
		outFile = outFile + ".png"
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
// something like: python3 -m http.server will work
func (pCtx *ProcessorContext) GotDebugPointCloud(pc pointcloud.PointCloud, name string) {
	outFile := filepath.Join(pCtx.d.out, name+"-"+filepath.Base(pCtx.currentFile))
	if !strings.HasSuffix(outFile, ".pcd") {
		outFile = outFile + ".pcd"
	}
	atomic.AddInt32(&pCtx.d.pendingImages, 1)
	go func() {
		f, err := os.OpenFile(outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			panic(err)
		}
		err = pc.ToPCD(f)
		if err != nil {
			panic(err)
		}

		atomic.AddInt32(&pCtx.d.pendingImages, -1)
	}()
	pCtx.output.getFile(pCtx.currentFile).addPCDCell(outFile, name)
}

// MultipleImageTestDebuggerProcessor TODO
type MultipleImageTestDebuggerProcessor interface {
	Process(
		t *testing.T,
		pCtx *ProcessorContext,
		fn string,
		img image.Image,
		logger golog.Logger,
	) error
}

// NewMultipleImageTestDebugger TODO
func NewMultipleImageTestDebugger(t *testing.T, prefix, glob string, imagesAligned bool) *MultipleImageTestDebugger {
	d := MultipleImageTestDebugger{logger: golog.NewTestLogger(t)}
	d.imagesAligned = imagesAligned
	d.glob = glob
	d.inroot = artifact.MustPath(prefix)
	d.name = prefix + "-" + t.Name()
	d.name = strings.Replace(d.name, "/", "-", 100)
	d.name = strings.Replace(d.name, " ", "-", 100)
	d.out = testutils.TempDirT(t, "", strings.ReplaceAll(prefix, "/", "_"))
	return &d
}

// Process TODO
func (d *MultipleImageTestDebugger) Process(t *testing.T, x MultipleImageTestDebuggerProcessor) (err error) {
	files, err := filepath.Glob(filepath.Join(d.inroot, d.glob))
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
		for _, f := range files {
			if !IsImageFile(f) {
				continue
			}

			numFiles++

			currentFile := f
			d.logger.Debug(currentFile)

			t.Run(filepath.Base(f), func(t *testing.T) {
				t.Parallel()
				img, err := readImageFromFile(currentFile, d.imagesAligned)
				test.That(t, err, test.ShouldBeNil)

				pCtx := &ProcessorContext{
					d:           d,
					currentFile: currentFile,
					output:      &d.output,
				}

				pCtx.GotDebugImage(img, "raw")

				logger := golog.NewTestLogger(t)
				err = x.Process(t, pCtx, currentFile, img, logger)
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

	outFile, err := os.OpenFile(htmlOutFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() {
		err = multierr.Combine(err, outFile.Close())
	}()
	return theTemplate.Execute(outFile, &d.output)
}
