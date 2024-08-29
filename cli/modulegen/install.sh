#!/bin/bash

if [[ $OS = "windows" ]]; then
	curl -sL -o ./cli/modulegen/.__module_gen https://github.com/viamrobotics/module-template-generator/releases/latest/download/windows-main.exe
elif [[ $OS = "darwin" && $ARCH = "arm64" ]]; then
	curl -sL -o ./cli/modulegen/.__module_gen https://github.com/viamrobotics/module-template-generator/releases/latest/download/macosx_arm64-main
elif [[ $OS = "darwin" && $ARCH = "amd64" ]]; then
	curl -sL -o ./cli/modulegen/.__module_gen https://github.com/viamrobotics/module-template-generator/releases/latest/download/macosx_x86_64-main
elif [[ $OS = "linux" && $ARCH = "arm64" ]]; then
	curl -sL -o ./cli/modulegen/.__module_gen https://github.com/viamrobotics/module-template-generator/releases/latest/download/linux_aarch64-main
elif [[ $OS = "linux" && $ARCH = "amd64" ]]; then
	curl -sL -o ./cli/modulegen/.__module_gen https://github.com/viamrobotics/module-template-generator/releases/latest/download/linux_x86_64-main
fi
