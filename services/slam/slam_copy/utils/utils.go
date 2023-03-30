// Package utils contains helper functions for the slam library implementations
package utils

import (
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/spatialmath"
)

// DictToString converts a dictionary to a string so
// it can be loaded into an arg for the slam process.
func DictToString(m map[string]string) string {
	stringMapList := make([]string, len(m))
	i := 0
	for k, val := range m {
		stringMapList[i] = k + "=" + val
		i++
	}
	stringMap := strings.Join(stringMapList, ",")

	return "{" + stringMap + "}"
}

// CheckQuaternionFromClientAlgo checks to see if the internal SLAM algorithm sent a quaternion. If it did, return the updated pose.
func CheckQuaternionFromClientAlgo(pose spatialmath.Pose, componentReference string,
	returnedExt map[string]interface{},
) (spatialmath.Pose, string, error) {
	// check if extra contains a quaternion. If no quaternion is found, throw an error
	if val, ok := returnedExt["quat"]; ok {
		q := val.(map[string]interface{})

		valReal, ok1 := q["real"].(float64)
		valIMag, ok2 := q["imag"].(float64)
		valJMag, ok3 := q["jmag"].(float64)
		valKMag, ok4 := q["kmag"].(float64)

		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, "", errors.Errorf("error getting SLAM position: quaternion given, but invalid format detected, %v", q)
		}
		actualPose := spatialmath.NewPose(pose.Point(),
			&spatialmath.Quaternion{Real: valReal, Imag: valIMag, Jmag: valJMag, Kmag: valKMag})
		return actualPose, componentReference, nil
	}
	return nil, "", errors.Errorf("error getting SLAM position: quaternion not given, %v", returnedExt)
}
