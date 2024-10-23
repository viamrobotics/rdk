package capture

import "errors"

// MongoConfig is the optional data capture mongo config.
type MongoConfig struct {
	URI        string `json:"uri"`
	Database   string `json:"database"`
	Collection string `json:"collection"`
}

// Equal returns true when both MongoConfigs are equal.
func (mc MongoConfig) Equal(o MongoConfig) bool {
	return mc.URI == o.URI && mc.Database == o.Database && mc.Collection == o.Collection
}

func (mc MongoConfig) validate() error {
	if mc.URI == "" {
		return errors.New("mongo config URI can't be empty string")
	}

	if mc.Database == "" {
		return errors.New("mongo config Database can't be empty string")
	}

	if mc.Collection == "" {
		return errors.New("mongo config Collection can't be empty string")
	}

	return nil
}

// Config is the capture config.
type Config struct {
	// CaptureDisabled if set to true disables all data capture collectors
	CaptureDisabled bool
	// CaptureDir defines where data capture should write capture files
	CaptureDir string
	// Tags defines the tags that should be added to capture file metadata
	Tags []string
	// MaximumCaptureFileSizeBytes defines the maximum size that in progress data capture
	// (.prog) files should be allowed to grow to before they are convered into .capture
	// files
	MaximumCaptureFileSizeBytes int64

	MongoConfig *MongoConfig
}
