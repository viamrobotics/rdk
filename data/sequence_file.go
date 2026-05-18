package data

import "time"

// SequencesDir is the subdir under captureDir that holds sequence files (.progseq, .seq).
const SequencesDir = "sequences"

// SequenceFile is the on-disk representation of a sequence.
type SequenceFile struct {
	StartTime    time.Time          `json:"start_time"`
	EndTime      time.Time          `json:"end_time"`
	Resources    []SequenceResource `json:"resources"`
	SequenceTags []string           `json:"sequence_tags,omitempty"`
}

// SequenceResource is the on-disk form of a resource/method pair in a sequence.
type SequenceResource struct {
	ResourceName string `json:"resource_name"`
	MethodName   string `json:"method_name"`
}
