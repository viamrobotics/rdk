<<<<<<< HEAD
<<<<<<< HEAD:components/camera/videosource/verify_main_test.go
package videosource
=======
package audioinput
>>>>>>> c59516e7b516ee489512669cb6f0564e308643c1:components/audioinput/verify_main_test.go
=======
package videosource
>>>>>>> c59516e7b516ee489512669cb6f0564e308643c1

import (
	"testing"

	testutilsext "go.viam.com/utils/testutils/ext"
)

// TestMain is used to control the execution of all tests run within this package (including _test packages).
func TestMain(m *testing.M) {
	testutilsext.VerifyTestMain(m)
}
