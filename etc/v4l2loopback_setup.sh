#!/bin/bash

set -euo pipefail

if [[ -z "$BASH_VERSION" ]] ; then
  echo "Please run this script ($0) with bash, or execute it directly" >&2
  exit 1
fi

if [[ "$(whoami)" != "root" ]]; then
  echo "Please do run this script directly as root" >&2
  exit 1
fi

if [[ ! "$(type apt-get)" ]]; then
  echo "Unable to find APT package handling utility (apt-get)"
  echo "Are you sure you're on a Debian-based Linux variant (e.g. Ubuntu)?"
  exit 1
fi

install_dependencies() {
  apt-get install --assume-yes kmod git make gcc libelf-dev
}

install_kernel_headers() {
  echo "downloading kernel headers..."
  # Source: https://gist.github.com/jperkin/c37a574379ef71e339361954be96be12
  if ! grep -q "Raspberry Pi" /proc/device-tree/model ; then
    apt-get --assume-yes install "linux-headers-$(uname -r)"
    echo "done"
    return
  fi

  apt install --assume-yes git bc bison flex libssl-dev python2 libncurses5-dev
  wget https://raw.githubusercontent.com/RPi-Distro/rpi-source/master/rpi-source -O /usr/local/bin/rpi-source
  chmod +x /usr/local/bin/rpi-source
  rpi-source -q --tag-update
  rpi-source

  echo "installing kernel headers..."
  echo "this may take a while"
  KERNEL_VERSION=$(uname -r)
  # using --ignore to avoid using all available cores and grinding the OS to a halt.
  make "-j$(nproc --ignore=1)" modules -C "/lib/modules/${KERNEL_VERSION}/source"
  make "-j$(nproc --ignore=1)" modules_install -C "/lib/modules/${KERNEL_VERSION}/source"
  echo "done"
}

install_v4l2loopback(){
  if [[ "$(type v4l2loopback-ctl)" ]];  then
    echo "v4l2loopback already installed"
    return
  fi

  apt-get install --assume-yes v4l2loopback-dkms v4l2loopback-utils
}

install_gstreamer(){
  if [[ "$(type gst-launch-1.0)" ]]; then
    echo "gstreamer already installed"
    return
  fi

  apt-get install --assume-yes libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev libgstreamer-plugins-bad1.0-dev
  apt-get install --assume-yes gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad
  apt-get install --assume-yes gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-x
  apt-get install --assume-yes gstreamer1.0-alsa gstreamer1.0-gl gstreamer1.0-gtk3 gstreamer1.0-qt5 gstreamer1.0-pulseaudio
}

allow_modprobe_as_superuser() {
  SUDOER_FILE=/etc/sudoers.d/v4l2loopback
  if grep -q "$SUDO_USER.*NOPASSWD.*modprobe" $SUDOER_FILE ; then
    echo "$SUDOER_FILE already allows $SUDO_USER to use modprobe without password"
    return
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
      echo "$SUDO_USER ALL = NOPASSWD: $(which modprobe)" | EDITOR='tee -a' visudo -f $SUDOER_FILE
      echo "Done."
      echo
      ;;
    *)
      echo "Abort. No changes were made"
      return
      ;;
  esac
}

apt-get update --assume-yes
# In order to build kernel modules (i.e. v4l2loopback) you must have the kernel headers installed
install_kernel_headers
install_dependencies
install_v4l2loopback
install_gstreamer
allow_modprobe_as_superuser
