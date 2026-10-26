package cli

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestMetadataReadActionRequiresIdentifier(t *testing.T) {
	cCtx, _, _, _ := setup(nil, nil, nil, nil, "token")

	err := MetadataReadAction(context.Background(), cCtx, metadataReadArgs{})
	test.That(t, err, test.ShouldNotBeNil)
	for _, flag := range []string{generalFlagOrgID, generalFlagLocationID, generalFlagMachineID, generalFlagPartID} {
		test.That(t, err.Error(), test.ShouldContainSubstring, "--"+flag)
	}
}
