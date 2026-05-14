package capture

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"go.viam.com/rdk/services/datamanager"
)

// Sequence file extensions, used by the data manager's capture control sensor flow.
//
// The lifecycle mirrors data capture files (.prog → .capture):
//
//	.progseq — written when a sequence opens; in-progress, owned by the running process
//	.seq     — written when a sequence closes; ready for the sync worker to upload
//
// On startup, orphaned .progseq files (left from a crashed previous process) are
// finalized to .seq with end_at = file mtime.
const (
	// InProgressSequenceFileExt is the extension for sequence files whose definition window
	// is still open (the sensor is still emitting them). Sync skips files with this extension.
	InProgressSequenceFileExt = ".progseq"
	// CompletedSequenceFileExt is the extension for closed sequence files ready for upload.
	// Sync picks up files with this extension and calls CreateSequence on each.
	CompletedSequenceFileExt = ".seq"
)

// openSequence is the in-memory state for a sequence whose definition window is currently
// open (the sensor is still emitting it). When the sensor stops emitting it, the sequence
// ends and becomes a PendingSequence to be written to disk and uploaded.
type openSequence struct {
	// id uniquely identifies this sequence across opens/closes. Used as the .progseq filename
	// on disk so the close path can remove it and the .seq finalization knows which file is which.
	id string
	// startAt is when the sensor first emitted this sequence.
	startAt time.Time
	// resources is the resource set included in this sequence, frozen at start time.
	resources []datamanager.ResourceMethod
	// tags are attached to the resulting sequence record, frozen at start time.
	tags []string
}

// openSequenceKey is the comparable identity of an open sequence. Two SequenceReadings
// with the same canonical resources and tags map to the same key, regardless of input order.
// Matches the collectorMetadata pattern of using a struct of strings as a map key.
type openSequenceKey struct {
	resources string
	tags      string
}

// OpenedSequence is returned from SetActiveSequences for each sequence that opened this tick.
// Callers persist it to disk as <ID>.progseq so the open state survives a process crash.
type OpenedSequence struct {
	ID        string
	StartAt   time.Time
	Resources []datamanager.ResourceMethod
	Tags      []string
}

// PendingSequence is returned from SetActiveSequences for each sequence that ended this tick.
// Callers persist it to disk as <ID>.seq (replacing the corresponding <ID>.progseq); the
// sync worker then uploads it via CreateSequence.
type PendingSequence struct {
	ID           string
	StartAt      time.Time
	EndAt        time.Time
	Resources    []datamanager.ResourceMethod
	SequenceTags []string
}

// SetActiveSequences updates the set of open sequences to match the sensor's latest readings.
// Returns sequences that opened this tick (newly seen) and ended this tick (previously open
// but absent from active now). Callers persist each of these to disk for crash durability.
//
// Must be called with collectorsMu held — callers should invoke this within the same
// critical section as SetCaptureConfigs so capture-state and sequence-state updates per
// tick are atomic.
func (c *Capture) SetActiveSequences(now time.Time, active []datamanager.SequenceReading) (opened []OpenedSequence, ended []PendingSequence) {
	activeKeys := make(map[openSequenceKey]struct{}, len(active))
	for _, s := range active {
		k := newOpenSequenceKey(s)
		activeKeys[k] = struct{}{}
		if _, exists := c.openSequences[k]; !exists {
			seq := &openSequence{
				id:        uuid.NewString(),
				startAt:   now,
				resources: slices.Clone(s.Resources),
				tags:      slices.Clone(s.SequenceTags),
			}
			c.openSequences[k] = seq
			opened = append(opened, OpenedSequence{
				ID:        seq.id,
				StartAt:   seq.startAt,
				Resources: seq.resources,
				Tags:      seq.tags,
			})
			c.logger.Infow("sequence started",
				"id", seq.id,
				"start_at", seq.startAt,
				"resource_count", len(seq.resources),
				"tags", seq.tags,
			)
		}
	}

	for k, seq := range c.openSequences {
		if _, stillActive := activeKeys[k]; stillActive {
			continue
		}
		ended = append(ended, PendingSequence{
			ID:           seq.id,
			StartAt:      seq.startAt,
			EndAt:        now,
			Resources:    seq.resources,
			SequenceTags: seq.tags,
		})
		c.logger.Infow("sequence ended",
			"id", seq.id,
			"start_at", seq.startAt,
			"end_at", now,
			"duration", now.Sub(seq.startAt),
			"resource_count", len(seq.resources),
			"tags", seq.tags,
		)
		delete(c.openSequences, k)
	}
	return opened, ended
}

// newOpenSequenceKey returns a comparable identity for a SequenceReading. Resources and tags
// are sorted to a canonical form so that input order doesn't affect identity.
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
