#!/bin/bash

if [ `whoami` == "root" ]; then
	echo "Please do not run this script directly as root. Use your normal development user account."
	exit 1
fi

if [ "`sudo whoami`x" != "rootx" ]; then
	echo "Cannot sudo to root. Please correct (install/configure sudo for your user) and try again."
	exit 1
fi


if [ "$(uname)" == "Linux" ]; then

	if [ "$(uname -m)" != "x86_64" ]; then
		echo "Automated dev environment setup is only supported on Linux/x86_64 or Darwin (Mac)."
		echo "If you need to build on a Raspberry Pi, please install the Viam RPi image."
		exit 1
	fi

	INSTALL_CMD=""
	if apt --version > /dev/null 2>&1; then
		# Debian/Ubuntu
		INSTALL_CMD="apt install --assume-yes build-essential procps curl file git"
	elif pacman --version > /dev/null 2>&1; then
		# Arch
		INSTALL_CMD="pacman -Sy --needed --noconfirm base-devel procps-ng curl git"
	elif yum --version > /dev/null 2>&1; then
		# Fedora/Redhat
		INSTALL_CMD="yum -y install procps-ng curl file git libxcrypt-compat && yum -y groupinstall 'Development Tools'"
	fi

	sudo bash -c "$INSTALL_CMD"

	if [ $? -ne 0 ]; then
		echo "Package installation failed when running:"
		echo "sudo bash -c \"$INSTALL_CMD\""
		exit 1
	fi

	cat > ~/.viamdevrc <<-EOS
	if [[ "\$VIAM_DEV_ENV"x == "x" ]]; then
		export VIAM_DEV_ENV=1
		eval "\$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
		export LIBRARY_PATH=/home/linuxbrew/.linuxbrew/lib
		export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
	fi
	EOS



elif [ "$(uname)" == "Darwin" ]; then

	if ! gcc --version >/dev/null 2>&1; then
		echo "Please finish the Xcode CLI tools installation then rerun this script."
		exit 1
	fi


	if [ "$(uname -m)" == "arm64" ]; then

		cat > ~/.viamdevrc <<-EOS
		if [[ "\$VIAM_DEV_ENV"x == "x" ]]; then
			export VIAM_DEV_ENV=1
			eval "\$(/opt/homebrew/bin/brew shellenv)"
			export LIBRARY_PATH=/opt/homebrew/lib
			export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
		fi
		EOS

  else # assuming x86_64, but untested

		cat > ~/.viamdevrc <<-EOS
		if [[ "\$VIAM_DEV_ENV"x == "x" ]]; then
			export VIAM_DEV_ENV=1
			eval "\$(/usr/local/bin/brew shellenv)"
			export LIBRARY_PATH=/usr/local/lib
			export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*
		fi
		EOS

	fi

fi


# Add dev environment variables to shells
grep -q viamdevrc ~/.bash_profile || echo "source ~/.viamdevrc" >> ~/.bash_profile
grep -q viamdevrc ~/.bashrc || echo "source ~/.viamdevrc" >> ~/.bashrc
grep -q viamdevrc ~/.zshrc || echo "source ~/.viamdevrc" >> ~/.zshrc


# Install brew
brew --version > /dev/null 2>&1 || bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" || exit 1

# Has to be after the install so the brew eval can run
source ~/.viamdevrc

brew bundle --file=- <<-EOS

tap  "bufbuild/buf"
tap  "viamrobotics/brews"
brew "gcc@5" #Needed for cgo
brew "make"
brew "cmake"
brew "pkgconfig"
brew "go"
brew "protobuf"
brew "buf"
brew "protoc-gen-go"
brew "protoc-gen-doc"
brew "protoc-gen-go-grpc"
brew "protoc-gen-grpc-web"
brew "protoc-gen-grpc-gateway"
brew "ts-protoc-gen"
brew "grpcurl"
brew "node"
brew "nlopt"
brew "libx11"
brew "libxext"
brew "libvpx"
brew "x264"

EOS

if [ $? -ne 0 ]; then
	exit 1
fi

echo "Brew installed software versions..."
brew list --version

git config --global --get-regexp url. > /dev/null
if [ $? -ne 0 ]; then
	git config --global url.ssh://git@github.com/.insteadOf https://github.com/
fi

echo -e "\033[0;32m""Dev environment setup is complete!""\033[0m"
echo -e "Don't forget to restart your shell, or execute: ""\033[41m""source ~/.viamdevrc""\033[0m"
exit 0

