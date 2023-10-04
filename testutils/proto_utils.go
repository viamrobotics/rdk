package testutils

import "go.viam.com/utils/protoutils"

// ToProtoMapIgnoreOmitEmpty is a helper to convert anything
// to a map to compare against a structpb.
func ToProtoMapIgnoreOmitEmpty(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
