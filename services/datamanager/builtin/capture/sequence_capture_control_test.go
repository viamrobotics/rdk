package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
)

// newCaptureForTest returns a Capture wired to a mock clock and a temp captureDir
// so tests can advance time deterministically and inspect on-disk sequence files.
func newCaptureForTest(t *testing.T) (*Capture, *clock.Mock) {
	t.Helper()
	mockClk := clock.NewMock()
	mockClk.Set(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC))
	c := New(mockClk, logging.NewTestLogger(t))
	c.captureDir = t.TempDir()
	return c, mockClk
}

func sequence(tags []string, resources ...datamanager.ResourceMethod) datamanager.SequenceReading {
	return datamanager.SequenceReading{SequenceTags: tags, Resources: resources}
}

func resources(name, method string) datamanager.ResourceMethod {
	return datamanager.ResourceMethod{ResourceName: name, MethodName: method}
}

// readSeqFiles reads and unmarshals every .seq file in c.captureDir/sequences/.
func readSeqFiles(t *testing.T, c *Capture) []data.SequenceFile {
	t.Helper()
	dir := filepath.Join(c.captureDir, data.SequencesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var files []data.SequenceFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != data.CompletedSequenceFileExt {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(dir, e.Name()))
		test.That(t, err, test.ShouldBeNil)
		var sf data.SequenceFile
		test.That(t, json.Unmarshal(bytes, &sf), test.ShouldBeNil)
		files = append(files, sf)
	}
	return files
}

func TestSetActiveSequences_NewEntryOpens(t *testing.T) {
	c, _ := newCaptureForTest(t)

	c.SetActiveSequences([]datamanager.SequenceReading{
		sequence([]string{"walking"}, resources("camera-1", "GetImages")),
	})

	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
}

func TestSetActiveSequences_SameEntryStaysOpenAcrossTicks(t *testing.T) {
	c, clk := newCaptureForTest(t)
	t0 := clk.Now()
	entry := sequence([]string{"walking"}, resources("camera-1", "GetImages"))

	c.SetActiveSequences([]datamanager.SequenceReading{entry})
	clk.Add(100 * time.Millisecond)
	c.SetActiveSequences([]datamanager.SequenceReading{entry})

	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
	for _, rec := range c.openSequences {
		test.That(t, rec.StartAt, test.ShouldEqual, t0)
	}
}

// Multiple sequences can be tracked at once and close independently.
// When one disappears, only it closes; the other keeps going. Verifies the per-sequence isolation.
func TestSetActiveSequences_EntryDisappears_Closes(t *testing.T) {
	c, clk := newCaptureForTest(t)
	t0 := clk.Now()
	entry := sequence([]string{"walking"}, resources("camera-1", "GetImages"))

	c.SetActiveSequences([]datamanager.SequenceReading{entry})
	clk.Add(30 * time.Second)
	t1 := clk.Now()
	c.SetActiveSequences(nil)

	test.That(t, len(c.openSequences), test.ShouldEqual, 0)

	files := readSeqFiles(t, c)
	test.That(t, len(files), test.ShouldEqual, 1)
	test.That(t, files[0].StartTime, test.ShouldEqual, t0)
	test.That(t, files[0].EndTime, test.ShouldEqual, t1)
	test.That(t, files[0].SequenceTags, test.ShouldResemble, []string{"walking"})
	test.That(t, files[0].Resources, test.ShouldResemble, []data.SequenceResource{
		{ResourceName: "camera-1", MethodName: "GetImages"},
	})
}

func TestSetActiveSequences_TwoConcurrent_OneDisappears(t *testing.T) {
	c, clk := newCaptureForTest(t)
	a := sequence([]string{"a"}, resources("camera-1", "GetImages"))
	b := sequence([]string{"b"}, resources("arm-1", "JointPositions"))

	c.SetActiveSequences([]datamanager.SequenceReading{a, b})
	test.That(t, len(c.openSequences), test.ShouldEqual, 2)

	clk.Add(10 * time.Second)
	c.SetActiveSequences([]datamanager.SequenceReading{b})

	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
	files := readSeqFiles(t, c)
	test.That(t, len(files), test.ShouldEqual, 1)
	test.That(t, files[0].SequenceTags, test.ShouldResemble, []string{"a"})
}

func TestSetActiveSequences_BackToBackIdenticalContent(t *testing.T) {
	c, clk := newCaptureForTest(t)
	entry := sequence([]string{"walking"}, resources("camera-1", "GetImages"))
	t0 := clk.Now()

	c.SetActiveSequences([]datamanager.SequenceReading{entry})
	clk.Add(5 * time.Second)
	c.SetActiveSequences([]datamanager.SequenceReading{entry})

	// Gap tick → sequence closes.
	clk.Add(100 * time.Millisecond)
	c.SetActiveSequences(nil)
	test.That(t, len(c.openSequences), test.ShouldEqual, 0)

	files := readSeqFiles(t, c)
	test.That(t, len(files), test.ShouldEqual, 1)
	test.That(t, files[0].StartTime, test.ShouldEqual, t0)

	// Same content again → fresh open with new start time.
	clk.Add(5 * time.Second)
	t3 := clk.Now()
	c.SetActiveSequences([]datamanager.SequenceReading{entry})
	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
	for _, rec := range c.openSequences {
		test.That(t, rec.StartAt, test.ShouldEqual, t3)
	}
}

func TestSetActiveSequences_EmptyActiveClosesAll(t *testing.T) {
	c, _ := newCaptureForTest(t)

	c.SetActiveSequences([]datamanager.SequenceReading{
		sequence([]string{"a"}, resources("camera-1", "GetImages")),
		sequence([]string{"b"}, resources("arm-1", "JointPositions")),
	})
	c.SetActiveSequences(nil)

	test.That(t, len(c.openSequences), test.ShouldEqual, 0)
	test.That(t, len(readSeqFiles(t, c)), test.ShouldEqual, 2)
}

// flushOpenSequences closes everything in-flight, writes their .seq files with EndAt set
// to the current clock, and removes the corresponding .progseq files.
func TestFlushOpenSequences(t *testing.T) {
	c, clk := newCaptureForTest(t)
	t0 := clk.Now()

	c.SetActiveSequences([]datamanager.SequenceReading{
		sequence([]string{"a"}, resources("camera-1", "GetImages")),
		sequence([]string{"b"}, resources("arm-1", "JointPositions")),
	})
	test.That(t, len(c.openSequences), test.ShouldEqual, 2)

	clk.Add(15 * time.Second)
	tEnd := clk.Now()
	c.flushOpenSequences()

	test.That(t, len(c.openSequences), test.ShouldEqual, 0)

	dir := filepath.Join(c.captureDir, data.SequencesDir)
	entries, err := os.ReadDir(dir)
	test.That(t, err, test.ShouldBeNil)
	var seqCount, progCount int
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case data.CompletedSequenceFileExt:
			seqCount++
		case data.InProgressSequenceFileExt:
			progCount++
		}
	}
	test.That(t, seqCount, test.ShouldEqual, 2)
	test.That(t, progCount, test.ShouldEqual, 0)

	files := readSeqFiles(t, c)
	test.That(t, len(files), test.ShouldEqual, 2)
	for _, sf := range files {
		test.That(t, sf.StartTime, test.ShouldEqual, t0)
		test.That(t, sf.EndTime, test.ShouldEqual, tEnd)
	}
}

func TestOpenSequenceKey(t *testing.T) {
	a := resources("camera-1", "GetImages")
	b := resources("arm-1", "JointPositions")

	for _, tc := range []struct {
		name string
		x, y datamanager.SequenceReading
		eq   bool
	}{
		{
			name: "resource_order_independent",
			x:    sequence([]string{"t"}, a, b),
			y:    sequence([]string{"t"}, b, a),
			eq:   true,
		},
		{
			name: "tag_order_independent",
			x:    sequence([]string{"alpha", "beta"}, a),
			y:    sequence([]string{"beta", "alpha"}, a),
			eq:   true,
		},
		{
			name: "different_tags",
			x:    sequence([]string{"alpha"}, a),
			y:    sequence([]string{"beta"}, a),
			eq:   false,
		},
		{
			name: "different_resources",
			x:    sequence([]string{"t"}, a),
			y:    sequence([]string{"t"}, b),
			eq:   false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kx := newOpenSequenceKey(tc.x)
			ky := newOpenSequenceKey(tc.y)
			if tc.eq {
				test.That(t, kx, test.ShouldResemble, ky)
			} else {
				test.That(t, kx, test.ShouldNotResemble, ky)
			}
		})
	}
}
