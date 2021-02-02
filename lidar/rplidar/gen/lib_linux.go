// +build linux

package rplidargen

// #cgo CXXFLAGS: -w -I${SRCDIR}/third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/src -I${SRCDIR}/third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include
// #cgo LDFLAGS: -lrplidar_sdk -lrt -lstdc++ -lpthread
import "C"
