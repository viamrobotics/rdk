# Camera Servers

## Dependencies
* [libhttpserver](https://github.com/etr/libhttpserver)
* [librealsense](https://github.com/IntelRealSense/librealsense)
  * https://github.com/IntelRealSense/librealsense/blob/master/doc/distribution_linux.md
    
## Installation Instructions

**If on Raspberry Pi (Debian):** `sudo apt install xorg-dev`

Installing `librealsense`
```bash
sudo apt install libglfw3-dev libusb-1.0-0-dev libgl1-mesa-dev libglu1-mesa-dev
git clone git@github.com:IntelRealSense/librealsense.git
cd librealsense
mkdir build && cd build
cmake ..
make -j 4
sudo make install
```
    
### Installing `libhttpserver`
```bash
sudo apt install libmicrohttpd-dev libtool
git clone git@github.com:etr/libhttpserver.git
cd libhttpserver
./bootstrap
mkdir build && cd build
../configure
make -j 4
sudo make install
```

### If none of that works, try this:
https://github.com/IntelRealSense/librealsense/blob/master/doc/libuvc_installation.md

## Make Intel Realsense service
```bash
sudo ln -s ~/work/robotcore/utils/camera_servers/intelrealserver /usr/local/bin
sudo ln -s ~/work/robotcore/utils/camera_servers/intelrealserver.service /etc/systemd/system/
sudo systemctl start intelrealserver
sudo systemctl enable intelrealserver
```