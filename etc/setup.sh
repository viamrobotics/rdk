#!/bin/bash

if [ `whoami` == "root" ]; then
	echo "Please do not run this script directly as root. Use your normal development user account."
	exit 1
fi

if [ "`sudo whoami`x" != "rootx" ]; then
	echo "Cannot sudo to root. Please correct (install/configure sudo for your user) and try again."
	exit 1
fi

do_piOS(){
	sudo bash <<-EOS
	# Basic tools
	apt-get update && apt-get install -y curl gpg git

	# Backports repo
	echo "deb http://deb.debian.org/debian $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)-backports main" > /etc/apt/sources.list.d/backports.list

	# Viam repo
	curl -s https://us-apt.pkg.dev/doc/repo-signing-key.gpg | gpg --yes --dearmor -o /usr/share/keyrings/viam-google.gpg
	echo "deb [signed-by=/usr/share/keyrings/viam-google.gpg] https://us-apt.pkg.dev/projects/static-file-server-310021 $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2) main" > /etc/apt/sources.list.d/viam-google.list

	# Node repo
	curl -s https://deb.nodesource.com/gpgkey/nodesource.gpg.key | gpg --yes --dearmor -o /usr/share/keyrings/nodesource.gpg
	echo "deb [signed-by=/usr/share/keyrings/nodesource.gpg] https://deb.nodesource.com/node_18.x $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2) main" > /etc/apt/sources.list.d/nodesource.list

	# Install most things
	apt-get update && apt-get install -y build-essential nodejs libnlopt-dev libx264-dev libtensorflowlite-dev ffmpeg libjpeg62-turbo-dev

	# Install Gostream dependencies
	sudo apt-get install -y --no-install-recommends libopus-dev libvpx-dev libx11-dev libxext-dev libopusfile-dev

	# Install backports
	apt-get install -y -t $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)-backports golang-go

	# Raspberry Pi support
	test "$(uname -m)" = "aarch64" && apt-get install -y libpigpio-dev

	# upx
	UPX_URL=https://github.com/upx/upx/releases/download/v4.0.2/upx-4.0.2-amd64_linux.tar.xz
	if [ "$(uname -m)" = "aarch64" ]; then
		UPX_URL=https://github.com/upx/upx/releases/download/v4.0.2/upx-4.0.2-arm64_linux.tar.xz
	fi
	curl -L "\$UPX_URL" | tar -C /usr/local/bin/ --strip-components=1 --wildcards -xJv '*/upx'

	# canon
	GOBIN=/usr/local/bin go install github.com/viamrobotics/canon@latest

	# license_finder
	apt-get install -y ruby && gem install license_finder
	EOS

	if [ $? -ne 0 ]; then
		echo "Package installation failed, please retry."
		exit 1
	fi

	check_gcloud_auth
}

do_linux(){
	if apt-get --version > /dev/null 2>&1; then
		# Debian/Ubuntu
		INSTALL_CMD="apt-get install --assume-yes build-essential procps curl file git debianutils"
	elif pacman --version > /dev/null 2>&1; then
		# Arch
		INSTALL_CMD="pacman -Sy --needed --noconfirm base-devel procps-ng curl git which"
	elif yum --version > /dev/null 2>&1; then
		# Fedora/Redhat
		INSTALL_CMD="yum -y install procps-ng curl git which libstdc++-static && yum -y groupinstall 'Development Tools'"
	fi

	sudo bash -c "$INSTALL_CMD"

	if [ $? -ne 0 ]; then
		echo "Package installation failed when running:"
		echo "sudo bash -c \"$INSTALL_CMD\""
		exit 1
	fi

	cat > ~/.viamdevrc <<-EOS
	eval "\$(/home/linuxbrew/.linuxbrew/bin/brew shellenv)"
	export LIBRARY_PATH=/home/linuxbrew/.linuxbrew/lib
	export LD_LIBRARY_PATH=/home/linuxbrew/.linuxbrew/lib
	export CGO_LDFLAGS=-L/home/linuxbrew/.linuxbrew/lib
	export CGO_CFLAGS=-I/home/linuxbrew/.linuxbrew/include
	EOS

	do_brew
	mod_profiles
	check_gcloud_auth
}

do_darwin(){
	if ! gcc --version >/dev/null 2>&1; then
		echo "Please finish the Xcode CLI tools installation then rerun this script."
		exit 1
	fi

	if [ "$(uname -m)" == "arm64" ]; then
		cat > ~/.viamdevrc <<-EOS
		eval "\$(/opt/homebrew/bin/brew shellenv)"
		export LIBRARY_PATH=/opt/homebrew/lib
		export CGO_LDFLAGS=-L/opt/homebrew/lib
		export CGO_CFLAGS=-I/opt/homebrew/include
		EOS
  	else # assuming x86_64, but untested
		cat > ~/.viamdevrc <<-EOS
		eval "\$(/usr/local/bin/brew shellenv)"
		export LIBRARY_PATH=/usr/local/lib
		EOS
	fi

	do_brew
	mod_profiles
	check_gcloud_auth
}

mod_profiles(){
	# Add dev environment variables to shells
	test -f ~/.bash_profile && ( grep -q viamdevrc ~/.bash_profile || echo "source ~/.viamdevrc" >> ~/.bash_profile )
	test -f ~/.bashrc && ( grep -q viamdevrc ~/.bashrc || echo "source ~/.viamdevrc" >> ~/.bashrc )
	test -f ~/.zprofile && ( grep -q viamdevrc ~/.zprofile || echo "source ~/.viamdevrc" >> ~/.zprofile )
	test -f ~/.zshrc && ( grep -q viamdevrc ~/.zshrc || echo "source ~/.viamdevrc" >> ~/.zshrc )
}

# This workaround is for https://viam.atlassian.net/browse/RSDK-526, without the application default credential file our tests will
# create goroutines that get leaked and fail. Once https://github.com/googleapis/google-cloud-go/issues/5430 is fixed we can remove this.
check_gcloud_auth(){
	APP_CREDENTIALS_DIR="$HOME/.config/gcloud"
	mkdir -p $APP_CREDENTIALS_DIR
	APP_CREDENTIALS_FILE="$APP_CREDENTIALS_DIR/application_default_credentials.json"	
	if [ ! -f "$APP_CREDENTIALS_FILE" ]; then
		echo "Missing gcloud application default credentials, this can cause goroutines to leak if not configured. Creating with empty config at $APP_CREDENTIALS_FILE"
		echo '{"client_id":"XXXX","client_secret":"XXXX","refresh_token":"XXXX","type":"authorized_user"}' > $APP_CREDENTIALS_FILE
	fi
}

do_brew(){
	# Install brew
	brew --version > /dev/null 2>&1 || bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" || exit 1

	# Has to be after the install so the brew eval can run
	source ~/.viamdevrc

	brew bundle -v --file=- <<-EOS
	# viam tap
	tap  "viamrobotics/brews"

	# pinned
	brew "go@1.20", link: true, conflicts_with: ["go"]
	brew "node@18", link: true, conflicts_with: ["node"]

	# unpinned
	brew "canon"
	brew "pkg-config"
	brew "nlopt-static"
	brew "x264", args: ["build-from-source"]
	brew "jpeg-turbo"
	brew "ffmpeg"
	brew "licensefinder"
	brew "opus"
	brew "opusfile"
	brew "libvpx"
	brew "tensorflowlite" # Needs to be last
	EOS

	if [ $? -ne 0 ]; then
		echo "Package installation failed when running brew command, please retry."
		exit 1
	fi

	# due to a missing bottle in homebrew, this has to be installed on its own
	brew install upx

	echo "Brew installed software versions..."
	brew list --version
}

# Main install routine

if [ "$(uname)" == "Linux" ]; then
	if [ "$(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)" == "bullseye" ] || [ "$(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)" == "bookworm" ]; then
		NO_PROFILE=1
		do_piOS || exit 1
	elif [ "$(uname -m)" == "x86_64" ]; then
		do_linux || exit 1
	else
		echo -e "\033[41m""Native dev environment is only supported on Debian/Bullseye or Bookworm (x86_64 and aarch64), but brew-based support is available for generic Linux/x86_64 and Darwin (MacOS).""\033[0m"
		exit 1
	fi
elif [ "$(uname)" == "Darwin" ]; then
	do_darwin || exit 1
fi

echo -e "\033[0;32m""Dev environment setup is complete!""\033[0m"
test -n "$NO_PROFILE" || echo -e "Don't forget to restart your shell, or execute: ""\033[41m""source ~/.viamdevrc""\033[0m"
exit 0
