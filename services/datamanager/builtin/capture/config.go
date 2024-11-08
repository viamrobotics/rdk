package capture

// MongoConfig is the optional data capture mongo config.
type MongoConfig struct {
	ConnectionString string `json:"connection_string"`
	Database         string `json:"database"`
	Collection       string `json:"collection"`
}

// Equal returns true when both MongoConfigs are equal.
func (mc MongoConfig) Equal(o MongoConfig) bool {
	return mc.ConnectionString == o.ConnectionString && mc.Database == o.Database && mc.Collection == o.Collection
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
