package capture

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
)

func newCaptureForTest(t *testing.T) *Capture {
	t.Helper()
	return New(clock.New(), logging.NewTestLogger(t))
}

func makeSeq(tags []string, resources ...datamanager.ResourceMethod) datamanager.SequenceReading {
	return datamanager.SequenceReading{SequenceTags: tags, Resources: resources}
}

func res(name, method string) datamanager.ResourceMethod {
	return datamanager.ResourceMethod{ResourceName: name, Method: method}
}

func TestSetActiveSequences_NewEntryOpens(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	opened, closed := c.SetActiveSequences(t0, []datamanager.SequenceReading{
		makeSeq([]string{"walking"}, res("camera-1", "GetImages")),
	})

	test.That(t, len(opened), test.ShouldEqual, 1)
	test.That(t, closed, test.ShouldBeEmpty)
	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
}

func TestSetActiveSequences_SameEntryNextTickStaysOpen(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(100 * time.Millisecond)

	entry := makeSeq([]string{"walking"}, res("camera-1", "GetImages"))
	opened, closed := c.SetActiveSequences(t0, []datamanager.SequenceReading{entry})
	test.That(t, len(opened), test.ShouldEqual, 1)
	test.That(t, closed, test.ShouldBeEmpty)

	opened, closed = c.SetActiveSequences(t1, []datamanager.SequenceReading{entry})
	test.That(t, opened, test.ShouldBeEmpty)
	test.That(t, closed, test.ShouldBeEmpty)
	test.That(t, len(c.openSequences), test.ShouldEqual, 1)

	// startAt should still be t0, not t1.
	for _, rec := range c.openSequences {
		test.That(t, rec.startAt, test.ShouldEqual, t0)
	}
}

func TestSetActiveSequences_EntryDisappears_Closes(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(30 * time.Second)

	entry := makeSeq([]string{"walking"}, res("camera-1", "GetImages"))
	c.SetActiveSequences(t0, []datamanager.SequenceReading{entry})

	_, closed := c.SetActiveSequences(t1, nil)
	test.That(t, len(closed), test.ShouldEqual, 1)
	test.That(t, closed[0].StartAt, test.ShouldEqual, t0)
	test.That(t, closed[0].EndAt, test.ShouldEqual, t1)
	test.That(t, closed[0].SequenceTags, test.ShouldResemble, []string{"walking"})
	test.That(t, closed[0].Resources, test.ShouldResemble, []datamanager.ResourceMethod{res("camera-1", "GetImages")})
	test.That(t, len(c.openSequences), test.ShouldEqual, 0)
}

func TestSetActiveSequences_TwoConcurrent_OneDisappears(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(10 * time.Second)

	seqA := makeSeq([]string{"a"}, res("camera-1", "GetImages"))
	seqB := makeSeq([]string{"b"}, res("arm-1", "JointPositions"))

	c.SetActiveSequences(t0, []datamanager.SequenceReading{seqA, seqB})
	test.That(t, len(c.openSequences), test.ShouldEqual, 2)

	// Drop seqA, keep seqB.
	_, closed := c.SetActiveSequences(t1, []datamanager.SequenceReading{seqB})
	test.That(t, len(closed), test.ShouldEqual, 1)
	test.That(t, closed[0].SequenceTags, test.ShouldResemble, []string{"a"})
	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
}

func TestSetActiveSequences_BackToBackIdenticalContent_ProducesTwoSequences(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(5 * time.Second)
	t2 := t1.Add(100 * time.Millisecond) // sensor returns sequences: []
	t3 := t2.Add(5 * time.Second)

	entry := makeSeq([]string{"walking"}, res("camera-1", "GetImages"))

	c.SetActiveSequences(t0, []datamanager.SequenceReading{entry})
	_, closed := c.SetActiveSequences(t1, []datamanager.SequenceReading{entry})
	test.That(t, closed, test.ShouldBeEmpty)

	// Gap tick: sequence closes.
	_, closed = c.SetActiveSequences(t2, nil)
	test.That(t, len(closed), test.ShouldEqual, 1)
	test.That(t, closed[0].StartAt, test.ShouldEqual, t0)
	test.That(t, closed[0].EndAt, test.ShouldEqual, t2)

	// New tick with same content: opens a fresh recording.
	_, closed = c.SetActiveSequences(t3, []datamanager.SequenceReading{entry})
	test.That(t, closed, test.ShouldBeEmpty)
	test.That(t, len(c.openSequences), test.ShouldEqual, 1)
	for _, rec := range c.openSequences {
		test.That(t, rec.startAt, test.ShouldEqual, t3)
	}
}

func TestSetActiveSequences_EmptyActiveClosesAll(t *testing.T) {
	c := newCaptureForTest(t)
	t0 := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Second)

	c.SetActiveSequences(t0, []datamanager.SequenceReading{
		makeSeq([]string{"a"}, res("camera-1", "GetImages")),
		makeSeq([]string{"b"}, res("arm-1", "JointPositions")),
	})

	_, closed := c.SetActiveSequences(t1, []datamanager.SequenceReading{})
	test.That(t, len(closed), test.ShouldEqual, 2)
	test.That(t, len(c.openSequences), test.ShouldEqual, 0)
}

func TestOpenSequenceKey_ResourceOrderIndependent(t *testing.T) {
	a := makeSeq([]string{"tag1"},
		res("camera-1", "GetImages"),
		res("arm-1", "JointPositions"),
	)
	b := makeSeq([]string{"tag1"},
		res("arm-1", "JointPositions"),
		res("camera-1", "GetImages"),
	)
	test.That(t, newOpenSequenceKey(a), test.ShouldEqual, newOpenSequenceKey(b))
}

func TestOpenSequenceKey_TagOrderIndependent(t *testing.T) {
	a := makeSeq([]string{"alpha", "beta"}, res("camera-1", "GetImages"))
	b := makeSeq([]string{"beta", "alpha"}, res("camera-1", "GetImages"))
	test.That(t, newOpenSequenceKey(a), test.ShouldEqual, newOpenSequenceKey(b))
}

func TestOpenSequenceKey_DifferentContentDifferentKey(t *testing.T) {
	a := makeSeq([]string{"alpha"}, res("camera-1", "GetImages"))
	b := makeSeq([]string{"beta"}, res("camera-1", "GetImages"))
	c := makeSeq([]string{"alpha"}, res("arm-1", "JointPositions"))

	test.That(t, newOpenSequenceKey(a), test.ShouldNotEqual, newOpenSequenceKey(b))
	test.That(t, newOpenSequenceKey(a), test.ShouldNotEqual, newOpenSequenceKey(c))
	test.That(t, newOpenSequenceKey(b), test.ShouldNotEqual, newOpenSequenceKey(c))
}
