package capture

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"go.viam.com/rdk/services/datamanager"
)

// Sequence file extensions, mirroring data capture's .prog → .capture.
const (
	// InProgressSequenceFileExt names open sequence files; sync skips them.
	InProgressSequenceFileExt = ".progseq"
	// CompletedSequenceFileExt names closed sequence files; sync uploads them.
	CompletedSequenceFileExt = ".seq"
)

// openSequenceKey is the comparable identity of an open sequence, derived from canonicalized
// resources and tags. Mirrors the collectorMetadata pattern.
type openSequenceKey struct {
	resources string
	tags      string
}

// OpenedSequence is a sequence whose entry is still in the sensor's readings.
// Used both as Capture's in-memory map value and the return value of SetActiveSequences.
type OpenedSequence struct {
	ID           string
	StartAt      time.Time
	Resources    []datamanager.ResourceMethod
	SequenceTags []string
}

// Sequence is returned by SetActiveSequences for each sequence that ended this tick.
type Sequence struct {
	ID           string
	StartAt      time.Time
	EndAt        time.Time
	Resources    []datamanager.ResourceMethod
	SequenceTags []string
}

// SetActiveSequences reconciles open sequences against the sensor's latest readings, returning
// any that opened or ended this tick. Must be called with collectorsMu held.
func (c *Capture) SetActiveSequences(now time.Time, active []datamanager.SequenceReading) (opened []OpenedSequence, ended []Sequence) {
	activeKeys := make(map[openSequenceKey]struct{}, len(active))
	for _, s := range active {
		k := newOpenSequenceKey(s)
		activeKeys[k] = struct{}{}
		if _, exists := c.openSequences[k]; !exists {
			seq := &OpenedSequence{
				ID:           uuid.NewString(),
				StartAt:      now,
				Resources:    slices.Clone(s.Resources),
				SequenceTags: slices.Clone(s.SequenceTags),
			}
			c.openSequences[k] = seq
			opened = append(opened, *seq)
			c.logger.Infow("sequence started",
				"start_at", seq.StartAt,
				"resources", seq.Resources,
				"tags", seq.SequenceTags,
			)
		}
	}

	for k, seq := range c.openSequences {
		if _, stillActive := activeKeys[k]; stillActive {
			continue
		}
		ended = append(ended, Sequence{
			ID:           seq.ID,
			StartAt:      seq.StartAt,
			EndAt:        now,
			Resources:    seq.Resources,
			SequenceTags: seq.SequenceTags,
		})
		c.logger.Infow("sequence ended",
			"start_at", seq.StartAt,
			"end_at", now,
			"resources", seq.Resources,
			"tags", seq.SequenceTags,
		)
		delete(c.openSequences, k)
	}
	return opened, ended
}

// newOpenSequenceKey returns a comparable identity for s, sorted so input order doesn't matter.
func newOpenSequenceKey(s datamanager.SequenceReading) openSequenceKey {
	resources := slices.Clone(s.Resources)
	slices.SortFunc(resources, func(a, b datamanager.ResourceMethod) int {
		if c := cmp.Compare(a.ResourceName, b.ResourceName); c != 0 {
			return c
		}
		return cmp.Compare(a.Method, b.Method)
	})
	tags := slices.Clone(s.SequenceTags)
	slices.Sort(tags)
	return openSequenceKey{
		resources: fmt.Sprintf("%+v", resources),
		tags:      fmt.Sprintf("%+v", tags),
	}
}
