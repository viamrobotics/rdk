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

	# Try for minimal (no node or web tooling) environment on pi/jetson/similar
	if [ "$(uname -m)" == "aarch64" ] && [ $(cat /etc/debian_version | cut -d. -f1) -ge 10 ]; then
		PKG_LIST="build-essential procps curl file git golang-go wasmer-dev libnlopt-dev libx264-dev"
		if [ -d "/sys/bus/platform/drivers/raspberrypi-firmware" ]; then
			PKG_LIST="$PKG_LIST libpigpio-dev"
		fi

		INSTALL_CMD="echo 'deb [trusted=yes] http://packages.viam.com/debian viam/' > /etc/apt/sources.list.d/viam.list && \
		apt-get update && \
		apt-get install --assume-yes $PKG_LIST"

		sudo bash -c "$INSTALL_CMD"

		if [ $? -ne 0 ]; then
			echo "Package installation failed when running:"
			echo "sudo bash -c \"$INSTALL_CMD\""
			exit 1
		fi

		git config --global --get-regexp url. > /dev/null
		if [ $? -ne 0 ]; then
			git config --global url.ssh://git@github.com/.insteadOf https://github.com/
		fi

		echo -e "\033[41m""Full dev environment is only supported on Linux/x86_64, Darwin (MacOS).""\033[0m"
		echo -e "\033[0;32m""Minimal environment installed. Go build/run/test should work, but web targets may fail.""\033[0m"
		exit 0

	elif [ "$(uname -m)" != "x86_64" ]; then
		echo -e "\033[41m""Automated dev environment (full) setup is only supported on Linux/x86_64 and Darwin (MacOS).""\033[0m"
		echo "Minimal environment setup is also available for Debian-based aarch64 systems. (Raspberry Pi, Nvidia Jetson, etc.)"
		exit 1
	fi

	INSTALL_CMD=""
	if apt-get --version > /dev/null 2>&1; then
		# Debian/Ubuntu
		INSTALL_CMD="apt-get install --assume-yes build-essential procps curl file git"
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
		export CC=gcc-11
		export CXX=g++-11
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

# unpinned
brew "make"
brew "cmake"
brew "pkgconfig"
brew "grpcurl"
brew "nlopt"
brew "x264"
brew "protoc-gen-grpc-web"
# pinned
brew "gcc@11"
brew "go@1.17"
brew "node@17"
brew "protobuf@3.19"
# viam tap
tap  "viamrobotics/brews"
brew "libwasmer@2.1"

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
