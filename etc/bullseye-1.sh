#!/usr/bin/env bash

set -euo pipefail

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
apt-get update && apt-get install -y build-essential nodejs libnlopt-dev libx264-dev ffmpeg libjpeg62-turbo-dev

apt-get install libtensorflowlite-dev && echo ok tflite || echo skipping tflite

# Install backports
apt-get install -y -t $(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)-backports golang-go
