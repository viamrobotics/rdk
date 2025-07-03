package main

import (
	"errors"
	"fmt"
	"unsafe"
)

/*
#cgo LDFLAGS: -L./lib -Wl,-rpath,./lib -lgenerator_api -lgenerator
#include "include/gen_api.h"
#include <stdlib.h>
*/
import "C"

type GeneratorArgs struct {
	className     string
	componentName string
	buildDir      string
	sourceDir     string
	outPath       string
}

func ModuleGenerate(args *GeneratorArgs) error {
	className := C.CString(args.className)
	componentName := C.CString(args.componentName)
	buildDir := C.CString(args.buildDir)
	sourceDir := C.CString(args.sourceDir)
	outPath := C.CString(args.outPath)

	defer C.free(unsafe.Pointer(className))
	defer C.free(unsafe.Pointer(componentName))
	defer C.free(unsafe.Pointer(buildDir))
	defer C.free(unsafe.Pointer(sourceDir))
	defer C.free(unsafe.Pointer(outPath))

	result := C.viam_cli_generate_cpp_module(className, componentName, buildDir, sourceDir, outPath)
	if result != C.int(0) {
		return errors.New("cli generate failed")
	}

	return nil
}

func main() {
	args := GeneratorArgs{
		className:     "MyCoolSensor",
		componentName: "Sensor",
		buildDir:      "/Users/lia.stratopoulos@viam.com/repos/viam/viam-cpp-sdk/build-llvm",
		sourceDir:     "/Users/lia.stratopoulos@viam.com/repos/viam/viam-cpp-sdk/src/viam/sdk/components/",
		outPath:       "temp.cpp",
	}

	err := ModuleGenerate(&args)
	if err != nil {
		fmt.Println(err.Error())
	}
}
