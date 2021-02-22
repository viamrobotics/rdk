// +build linux

package mtigen

// #cgo CXXFLAGS: -std=c++11 -w -I${SRCDIR}/third_party/xspublic
// #cgo LDFLAGS: -L${SRCDIR}/third_party/xspublic/xscontroller -L${SRCDIR}/third_party/xspublic/xscommon -L${SRCDIR}/third_party/xspublic/xstypes -lxscontroller -lxscommon -lxstypes -lpthread -lrt -ldl
import "C"
