Dependencies
* https://github.com/etr/libhttpserver
* https://github.com/IntelRealSense/librealsense
  * https://github.com/IntelRealSense/librealsense/blob/master/doc/distribution_linux.md


Installing librealsense
    sudo apt install libglfw3-dev libusb-1.0-0-dev libgl1-mesa-dev libglu1-mesa-dev
    git clone git@github.com:IntelRealSense/librealsense.git
    cd librealsense
    mkdir build && cd build
    cmake ..
    make -j 4
    sudo make install
    
Installing libhttpserver
    sudo apt install libmicrohttpd-dev
    git clone git@github.com:etr/libhttpserver.git
    cd libhttpserver
    ./bootstrap
    mkdir build && cd build
    ../configure
    make -j 4
    sudo make install

make realsense service
    sudo ln -s ~/work/robotcore/utils/camera_servers/intelrealserver /usr/local/bin
    sudo ln -s ~/work/robotcore/utils/camera_servers/intelrealserver.service /etc/systemd/system/
    sudo systemctl start intelrealserver
    sudo systemctl enable intelrealserver
