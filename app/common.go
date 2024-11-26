package app

import "time"

// Constants used throughout app.
const (
	UploadChunkSize = 64 * 1024 // UploadChunkSize is 64 KB
	locationID      = "location_id"
	tag             = "tag"
	robotID         = "robot_id"
	partID          = "part_id"
	robotName       = "robot_name"
	partName        = "part_name"
	host            = "host_name"
	email           = "email"
	secret          = "secret"
	fragmentID      = "fragment_id"
)

// Variables used throughout app testing.
var (
	organizationID = "organization_id"
	start          = time.Now().UTC().Round(time.Millisecond)
	end            = time.Now().UTC().Round(time.Millisecond)
	tags           = []string{tag}
	limit          = 2
	pbLimit        = uint64(limit)
)
