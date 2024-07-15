package utils

import (
	goutils "go.viam.com/utils/rpc"
)

// CredentialsType signifies a means of representing a credential. For example,
// an API key.
type CredentialsType string

const (
	// CredentialsTypeRobotSecret is for credentials used against the cloud managing this robot.
	CredentialsTypeRobotSecret = "robot-secret"

	// CredentialsTypeRobotLocationSecret is for credentials used against the cloud managing this robot's location.
	CredentialsTypeRobotLocationSecret = "robot-location-secret"

	// CredentialsTypeAPIKey is intended for by external users, human and computer.
	CredentialsTypeAPIKey = goutils.CredentialsTypeAPIKey

	// CredentialsTypeExternal is for credentials that are to be produced by some
	// external authentication endpoint (see ExternalAuthService#AuthenticateTo) intended
	// for another, different consumer at a different endpoint.
	CredentialsTypeExternal = goutils.CredentialsTypeExternal
)

// Credentials packages up both a type of credential along with its payload which
// is formatted specific to the type.
type Credentials = goutils.Credentials

// WithEntityCredentials returns a DialOption which sets the entity credentials
// to use for authenticating the request. This is mutually exclusive with
// WithCredentials.
func WithEntityCredentials(entity string, creds goutils.Credentials) goutils.DialOption {
	return goutils.WithEntityCredentials(entity, creds)
}
