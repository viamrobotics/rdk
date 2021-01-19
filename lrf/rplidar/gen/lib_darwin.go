package rplidargen

// +build darwin

// #cgo CXXFLAGS: -I${SRCDIR}/third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/src -I${SRCDIR}/third_party/rplidar_sdk-release-v1.12.0/sdk/sdk/include
// #cgo LDFLAGS: -lrplidar_sdk -lstdc++ -lpthread
import "C"
