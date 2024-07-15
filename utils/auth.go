package utils

// CredentialsType signifies a means of representing a credential. For example,
// an API key.
type CredentialsType string

const (
	// CredentialsTypeRobotSecret is for credentials used against the cloud managing this robot.
	CredentialsTypeRobotSecret = "robot-secret"

	// CredentialsTypeRobotLocationSecret is for credentials used against the cloud managing this robot's location.
	CredentialsTypeRobotLocationSecret = "robot-location-secret"
)

// Credentials packages up both a type of credential along with its payload which
// is formatted specific to the type.
type Credentials struct {
	Type    CredentialsType `json:"type"`
	Payload string          `json:"payload"`
}
