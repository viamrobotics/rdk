package testutils

import "go.viam.com/utils/protoutils"

// ToProtoMapIgnoreOmitEmpty is a helper to convert an interface
// to a map to compare against a structpb.
func ToProtoMapIgnoreOmitEmpty(data interface{}) map[string]interface{} {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
