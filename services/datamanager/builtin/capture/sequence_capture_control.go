package capture

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/datamanager"
)

// openSequenceKey is the comparable identity of an open sequence, derived from canonicalized
// resources and tags. Stored as strings because Go map keys must be comparable.
type openSequenceKey struct {
	resources string
	tags      string
}

// OpenSequence is a sequence whose entry is still in the sensor's readings.
type OpenSequence struct {
	ID           string
	StartAt      time.Time
	Resources    []datamanager.ResourceMethod
	SequenceTags []string
}

// ClosedSequence is a sequence that ended and is awaiting upload.
type ClosedSequence struct {
	ID           string
	StartAt      time.Time
	EndAt        time.Time
	Resources    []datamanager.ResourceMethod
	SequenceTags []string
}

// SetActiveSequences reconciles open sequences against the sensor's latest readings.
// Newly-opened sequences are written to <id>.progseq; ended ones are written to <id>.seq
// (and the matching .progseq is removed).
func (c *Capture) SetActiveSequences(active []datamanager.SequenceReading) {
	now := c.clk.Now()
	activeKeys := make(map[openSequenceKey]struct{}, len(active))
	for _, s := range active {
		k, ok := newOpenSequenceKey(s, c.logger)
		if !ok {
			continue
		}
		activeKeys[k] = struct{}{}
		if _, exists := c.openSequences[k]; !exists {
			seq := &OpenSequence{
				ID:           uuid.NewString(),
				StartAt:      now,
				Resources:    slices.Clone(s.Resources),
				SequenceTags: slices.Clone(s.SequenceTags),
			}
			c.openSequences[k] = seq
			c.logger.Infow("sequence started",
				"start_at", seq.StartAt,
				"resources", seq.Resources,
				"tags", seq.SequenceTags,
			)
			if err := writeOpenSequence(c.captureDir, *seq); err != nil {
				c.logger.Errorw("failed to persist open sequence",
					"error", err, "id", seq.ID, "start_at", seq.StartAt)
			}
		}
	}

	for k, seq := range c.openSequences {
		if _, stillActive := activeKeys[k]; stillActive {
			continue
		}
		closed := ClosedSequence{
			ID:           seq.ID,
			StartAt:      seq.StartAt,
			EndAt:        now,
			Resources:    seq.Resources,
			SequenceTags: seq.SequenceTags,
		}
		c.logger.Infow("sequence ended",
			"start_at", seq.StartAt,
			"end_at", now,
			"resources", seq.Resources,
			"tags", seq.SequenceTags,
		)
		if err := writeClosedSequence(c.captureDir, closed); err != nil {
			c.logger.Errorw("failed to persist closed sequence",
				"error", err, "id", closed.ID, "start_at", closed.StartAt, "end_at", closed.EndAt)
		}
		delete(c.openSequences, k)
	}
}

// flushOpenSequences closes all in-flight sequences and persists them as .seq. Called from
// Capture.Close so a clean shutdown produces .seq files instead of orphan .progseq files.
func (c *Capture) flushOpenSequences() {
	c.collectorsMu.Lock()
	defer c.collectorsMu.Unlock()
	now := c.clk.Now()
	for k, seq := range c.openSequences {
		closed := ClosedSequence{
			ID:           seq.ID,
			StartAt:      seq.StartAt,
			EndAt:        now,
			Resources:    seq.Resources,
			SequenceTags: seq.SequenceTags,
		}
		c.logger.Infow("sequence ended (flush)",
			"start_at", seq.StartAt,
			"end_at", now,
			"resources", seq.Resources,
			"tags", seq.SequenceTags,
		)
		if err := writeClosedSequence(c.captureDir, closed); err != nil {
			c.logger.Errorw("failed to persist flushed sequence",
				"error", err, "id", closed.ID, "start_at", closed.StartAt, "end_at", closed.EndAt)
		}
		delete(c.openSequences, k)
	}
}

// newOpenSequenceKey returns a comparable identity for s, sorted so input order doesn't matter.
// Returns ok=false if the inputs can't be marshaled into a stable key; callers should drop the
// sequence rather than risk colliding distinct sequences under a degenerate key.
func newOpenSequenceKey(s datamanager.SequenceReading, logger logging.Logger) (openSequenceKey, bool) {
	resources := slices.Clone(s.Resources)
	slices.SortFunc(resources, func(a, b datamanager.ResourceMethod) int {
		if c := cmp.Compare(a.ResourceName, b.ResourceName); c != 0 {
			return c
		}
		return cmp.Compare(a.MethodName, b.MethodName)
	})
	tags := slices.Clone(s.SequenceTags)
	slices.Sort(tags)
	resourcesJSON, err := json.Marshal(resources)
	if err != nil {
		logger.Errorw("failed to marshal sequence resources for key; dropping sequence",
			"error", err, "resources", resources)
		return openSequenceKey{}, false
	}
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		logger.Errorw("failed to marshal sequence tags for key; dropping sequence",
			"error", err, "tags", tags)
		return openSequenceKey{}, false
	}
	return openSequenceKey{
		resources: string(resourcesJSON),
		tags:      string(tagsJSON),
	}, true
}
