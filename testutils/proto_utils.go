package testutils

import "go.viam.com/utils/protoutils"

func ToProtoMapIgnoreOmitEmpty(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
