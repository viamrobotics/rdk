#!/bin/bash

set -euo pipefail

if [ "$(whoami)" == "root" ]; then
	echo "Please do not run this script directly as root. Use your normal development user account."
	exit 1
fi

if ! apt-get --version >/dev/null 2>&1; then
  echo "Unable to find APT package handling utility (apt-get)"
  echo "Are you sure you're on a Debian-based Linux variant (e.g. Ubuntu)?"
  exit 1
fi

install_dependencies() {
  sudo bash -c "apt-get install --assume-yes kmod git make gcc libelf-dev"
}

install_kernel_headers() {
  echo "updating kernel module..."
  sudo bash <<-EOS
	sudo apt-get --assume-yes update
	sudo apt-get --assume-yes dist-upgrade
	sudo apt-get --assume-yes upgrade
	EOS
	echo "done"

	if [ -f /var/run/reboot-required ]; then
      echo "Reboot required!"
      echo "Run script after reboot"
      exit 1
  fi

  echo "installing kernel headers..."
  sudo apt-get --assume-yes install "linux-headers-$(uname -r)" || sudo apt-get --assume-yes install linux-headers
  echo "done"
}

install_v4l2loopback(){
  if command -v v4l2loopback-ctl > /dev/null 2>&1; then
    echo "v4l2loopback already installed"
    return
  fi

  sudo bash <<-EOS
	apt-get install --assume-yes v4l2loopback-dkms v4l2loopback-utils
	EOS
}

install_gstreamer(){
  if command -v gst-launch-1.0 --version > /dev/null 2>&1; then
    echo "gstreamer already installed"
    return
  fi

  sudo bash <<-EOS
	apt-get install --assume-yes libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev libgstreamer-plugins-bad1.0-dev
	apt-get install --assume-yes gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad
	apt-get install --assume-yes gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-x
	apt-get install --assume-yes gstreamer1.0-alsa gstreamer1.0-gl gstreamer1.0-gtk3 gstreamer1.0-qt5 gstreamer1.0-pulseaudio
	EOS
}

allow_modprobe_as_superuser() {
  SUDOER_FILE=/etc/sudoers.d/v4l2loopback
  if [ -f $SUDOER_FILE ]; then
    echo "$SUDOER_FILE already exists"
    exit
  fi

  echo "$SUDOER_FILE not found"
  echo "'modprobe', which requires sudo permissions, is needed to load kernel modules."
  echo "To run 'sudo modprobe' without requesting a password from stdin (essential for automated tasks like go test) it's recommended you give the current user permission to do so"
  echo "Attempting to do so now..."
  printf "Do you want to continue? [Y/n] "
  read -r response
  case $response in
    [yY][eE][sS]|[yY]|'')
      echo
      echo "Allowing the current user to run 'sudo modprobe' without a password...";
      echo "$USER ALL = NOPASSWD: $(which modprobe)" | sudo EDITOR='tee -a' visudo -f $SUDOER_FILE
      echo "Done."
      echo
      ;;
    *)
      echo "Abort. No changes were made"
      return
      ;;
  esac
}

# In order to build kernel modules (i.e. v4l2loopback) you must have the kernel headers installed
install_kernel_headers
install_dependencies
install_v4l2loopback
install_gstreamer
allow_modprobe_as_superuser
