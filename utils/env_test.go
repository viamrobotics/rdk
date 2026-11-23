package utils

import (
	"fmt"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestLogViamEnvVariablesMasksAPIKey(t *testing.T) {
	t.Setenv(APIKeyEnvVar, "process-env-secret")
	t.Setenv("VIAM_Api_Key", "mixed-case-secret")
	t.Setenv(APIKeyIDEnvVar, "key-id")
	logger, logs := logging.NewObservedTestLogger(t)

	LogViamEnvVariables("env", map[string]string{
		APIKeyEnvVar:        "map-secret",
		MachinePartIDEnvVar: "part-1",
	}, logger)

	entries := logs.All()
	test.That(t, entries, test.ShouldHaveLength, 1)
	logged := fmt.Sprint(entries[0].ContextMap()["environment"])
	for _, secret := range []string{"process-env-secret", "mixed-case-secret", "map-secret"} {
		test.That(t, logged, test.ShouldNotContainSubstring, secret)
	}
	test.That(t, logged, test.ShouldContainSubstring, APIKeyEnvVar+"=XXXXXXXXXX")
	test.That(t, logged, test.ShouldContainSubstring, APIKeyIDEnvVar+"=key-id")
	test.That(t, logged, test.ShouldContainSubstring, MachinePartIDEnvVar+"=part-1")
}
